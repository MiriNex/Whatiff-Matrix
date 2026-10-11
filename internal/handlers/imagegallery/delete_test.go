package imagegallery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theimaginaryfoundation/what-iff/internal/datastore"
	"github.com/theimaginaryfoundation/what-iff/internal/middleware"
	"github.com/theimaginaryfoundation/what-iff/internal/models"
	"github.com/theimaginaryfoundation/what-iff/internal/storage"
	"go.uber.org/zap"
)

// fakeDeleteStore holds one user's rows; anything else is not found, as in the datastore.
type fakeDeleteStore struct {
	Store
	mu      sync.Mutex
	owner   uuid.UUID
	rows    map[uuid.UUID]models.FileAttachment
	deleted []uuid.UUID
}

func (f *fakeDeleteStore) GetFileAttachment(_ context.Context, userID, id uuid.UUID) (*models.FileAttachment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok || userID != f.owner {
		return nil, datastore.ErrFileAttachmentNotFound
	}
	return &r, nil
}

func (f *fakeDeleteStore) DeleteFileAttachment(_ context.Context, _ uuid.UUID, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rows, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeDeleteStore) FileAttachmentProviderFileShared(context.Context, uuid.UUID, string) (bool, error) {
	return false, nil
}

func (f *fakeDeleteStore) ReferencedFileAttachmentKeys(context.Context, []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (f *fakeDeleteStore) ExistingFileAttachmentIDs(context.Context, []uuid.UUID) (map[uuid.UUID]bool, error) {
	return map[uuid.UUID]bool{}, nil
}

// failingFileStore refuses to delete one key.
type failingFileStore struct {
	mu      sync.Mutex
	failKey string
	deleted []string
}

func (s *failingFileStore) UploadFile(context.Context, string, []byte, string) error { return nil }
func (s *failingFileStore) DownloadFile(context.Context, string) ([]byte, error)     { return nil, nil }
func (s *failingFileStore) DeleteFile(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key == s.failKey {
		return errors.New("storage down")
	}
	s.deleted = append(s.deleted, key)
	return nil
}

func serveAs(t *testing.T, userID uuid.UUID, store Store, files storage.FileStore, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(store, zap.NewNop(), files)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestDeleteFilesReportsEachID(t *testing.T) {
	user := uuid.New()
	img := models.FileAttachment{ID: uuid.New(), FileType: "image/png", S3Key: "users/u/a.png"}
	doc := models.FileAttachment{ID: uuid.New(), FileType: "text/markdown", S3Key: "users/u/b.md"}
	stuck := models.FileAttachment{ID: uuid.New(), FileType: "image/png", S3Key: "users/u/stuck.png"}
	store := &fakeDeleteStore{owner: user, rows: map[uuid.UUID]models.FileAttachment{img.ID: img, doc.ID: doc, stuck.ID: stuck}}
	files := &failingFileStore{failKey: "users/u/stuck.png"}
	stranger := uuid.New()

	body, _ := json.Marshal(map[string]any{"ids": []uuid.UUID{img.ID, stranger, doc.ID, stuck.ID}})
	rr := serveAs(t, user, store, files, http.MethodPost, "/image-gallery/delete", string(body))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{
		"deleted": ["`+img.ID.String()+`", "`+doc.ID.String()+`"],
		"failed": [
			{"id": "`+stranger.String()+`", "error": "not found"},
			{"id": "`+stuck.ID.String()+`", "error": "could not delete the stored image"}
		]
	}`, rr.Body.String())
	assert.ElementsMatch(t, []uuid.UUID{img.ID, doc.ID}, store.deleted)
	assert.Contains(t, files.deleted, "users/u/a.png", "an image loses its stored object, as on DELETE /image-gallery/{id}")
}

func TestDeleteFilesForSomeoneElsesFilesDeletesNothing(t *testing.T) {
	owner := uuid.New()
	img := models.FileAttachment{ID: uuid.New(), FileType: "image/png"}
	store := &fakeDeleteStore{owner: owner, rows: map[uuid.UUID]models.FileAttachment{img.ID: img}}

	rr := serveAs(t, uuid.New(), store, nil, http.MethodPost, "/image-gallery/delete", `{"ids":["`+img.ID.String()+`"]}`)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"deleted":[],"failed":[{"id":"`+img.ID.String()+`","error":"not found"}]}`, rr.Body.String())
	assert.Empty(t, store.deleted)
}

func TestDeleteFilesRefusesBadRequests(t *testing.T) {
	tooMany := make([]uuid.UUID, maxBatchIDs+1)
	for i := range tooMany {
		tooMany[i] = uuid.New()
	}
	tooManyBody, _ := json.Marshal(map[string]any{"ids": tooMany})
	for name, body := range map[string]string{
		"not json":     `nope`,
		"no ids":       `{"ids":[]}`,
		"missing ids":  `{}`,
		"bad id":       `{"ids":["not-a-uuid"]}`,
		"too many ids": string(tooManyBody),
	} {
		t.Run(name, func(t *testing.T) {
			store := &fakeDeleteStore{owner: uuid.New(), rows: map[uuid.UUID]models.FileAttachment{}}
			rr := serveAs(t, store.owner, store, nil, http.MethodPost, "/image-gallery/delete", body)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Empty(t, store.deleted)
		})
	}
}

func TestDeleteFilesNeedsASignedInUser(t *testing.T) {
	h := NewHandler(&fakeDeleteStore{}, zap.NewNop(), nil)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/image-gallery/delete", strings.NewReader(`{"ids":["`+uuid.NewString()+`"]}`)))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// The single-image route kept its statuses when its body moved into filedelete.
func TestDeleteImageStatuses(t *testing.T) {
	user := uuid.New()
	img := models.FileAttachment{ID: uuid.New(), FileType: "image/png", S3Key: "users/u/a.png"}
	stuck := models.FileAttachment{ID: uuid.New(), FileType: "image/png", S3Key: "users/u/stuck.png"}
	doc := models.FileAttachment{ID: uuid.New(), FileType: "application/pdf"}
	tests := []struct {
		name string
		id   string
		want int
	}{
		{"deletes an image", img.ID.String(), http.StatusNoContent},
		{"not the caller's or missing", uuid.NewString(), http.StatusNotFound},
		{"not an image", doc.ID.String(), http.StatusBadRequest},
		{"stored object will not delete", stuck.ID.String(), http.StatusInternalServerError},
		{"bad id", "not-a-uuid", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeDeleteStore{owner: user, rows: map[uuid.UUID]models.FileAttachment{img.ID: img, stuck.ID: stuck, doc.ID: doc}}
			rr := serveAs(t, user, store, &failingFileStore{failKey: "users/u/stuck.png"}, http.MethodDelete, "/image-gallery/"+tt.id, "")
			assert.Equal(t, tt.want, rr.Code)
		})
	}
}
