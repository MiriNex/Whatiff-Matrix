package filedelete

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theimaginaryfoundation/what-iff/internal/datastore"
	"github.com/theimaginaryfoundation/what-iff/internal/models"
	"github.com/theimaginaryfoundation/what-iff/internal/storage"
	"go.uber.org/zap"
)

// memStore is an in-memory attachment table owned per user; reference answers come from the rows
// still present, as they would from the datastore.
type memStore struct {
	mu        sync.Mutex
	rows      map[uuid.UUID]models.FileAttachment
	owners    map[uuid.UUID]uuid.UUID
	getErr    map[uuid.UUID]error
	deleteErr map[uuid.UUID]error
	sharedErr error
}

func newMemStore() *memStore {
	return &memStore{
		rows:      map[uuid.UUID]models.FileAttachment{},
		owners:    map[uuid.UUID]uuid.UUID{},
		getErr:    map[uuid.UUID]error{},
		deleteErr: map[uuid.UUID]error{},
	}
}

func (m *memStore) add(owner uuid.UUID, att models.FileAttachment) models.FileAttachment {
	m.mu.Lock()
	defer m.mu.Unlock()
	if att.ID == uuid.Nil {
		att.ID = uuid.New()
	}
	m.rows[att.ID] = att
	m.owners[att.ID] = owner
	return att
}

func (m *memStore) has(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.rows[id]
	return ok
}

func (m *memStore) GetFileAttachment(_ context.Context, userID, id uuid.UUID) (*models.FileAttachment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.getErr[id]; err != nil {
		return nil, err
	}
	r, ok := m.rows[id]
	if !ok || m.owners[id] != userID {
		return nil, datastore.ErrFileAttachmentNotFound
	}
	return &r, nil
}

func (m *memStore) DeleteFileAttachment(_ context.Context, userID, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.deleteErr[id]; err != nil {
		return err
	}
	if m.owners[id] != userID {
		return datastore.ErrFileAttachmentNotFound
	}
	delete(m.rows, id)
	return nil
}

func (m *memStore) FileAttachmentProviderFileShared(_ context.Context, id uuid.UUID, fileID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sharedErr != nil {
		return false, m.sharedErr
	}
	for rid, r := range m.rows {
		if rid != id && r.FileID != nil && *r.FileID == fileID {
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) ReferencedFileAttachmentKeys(_ context.Context, keys []string) (map[string]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]bool{}
	for _, k := range keys {
		for _, r := range m.rows {
			if r.S3Key == k {
				out[k] = true
			}
		}
	}
	return out, nil
}

func (m *memStore) ExistingFileAttachmentIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		if _, ok := m.rows[id]; ok {
			out[id] = true
		}
	}
	return out, nil
}

// memFiles records object deletes; a key in failOn errors.
type memFiles struct {
	mu      sync.Mutex
	deleted []string
	failOn  map[string]bool
}

func (f *memFiles) UploadFile(context.Context, string, []byte, string) error { return nil }
func (f *memFiles) DownloadFile(context.Context, string) ([]byte, error)     { return nil, nil }
func (f *memFiles) DeleteFile(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn[key] {
		return errors.New("storage down")
	}
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *memFiles) deletedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string(nil), f.deleted...)
	sort.Strings(out)
	return out
}

type memProvider struct {
	mu      sync.Mutex
	deleted []string
	err     error
}

func (p *memProvider) DeleteProviderFileAttachment(_ context.Context, fileID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.deleted = append(p.deleted, fileID)
	return nil
}

func strPtr(s string) *string { return &s }

func TestDeleteByKind(t *testing.T) {
	user := uuid.New()
	tests := []struct {
		name          string
		att           models.FileAttachment
		setup         func(*memStore, *memFiles, *memProvider, models.FileAttachment)
		wantStage     Stage
		wantRowGone   bool
		wantKeys      func(models.FileAttachment) []string
		wantProviders []string
	}{
		{
			name:        "image: full object, thumbnail, then row",
			att:         models.FileAttachment{FileType: "image/png", S3Key: "users/u/img.png", Name: "img.png"},
			wantRowGone: true,
			wantKeys: func(a models.FileAttachment) []string {
				return []string{"users/u/img.png", storage.FileKeyForImageThumbnail(user, a.ID)}
			},
		},
		{
			name:        "image without a stored key uses the derived one",
			att:         models.FileAttachment{FileType: "image/jpeg", Name: "old.jpg"},
			wantRowGone: true,
			wantKeys: func(a models.FileAttachment) []string {
				return []string{storage.FileKeyForImage(user, a.ID, a.Name), storage.FileKeyForImageThumbnail(user, a.ID)}
			},
		},
		{
			name: "image whose full object cannot be deleted keeps its row",
			att:  models.FileAttachment{FileType: "image/png", S3Key: "users/u/stuck.png"},
			setup: func(_ *memStore, f *memFiles, _ *memProvider, _ models.FileAttachment) {
				f.failOn = map[string]bool{"users/u/stuck.png": true}
			},
			wantStage: StageStorage,
		},
		{
			name: "image thumbnail failure does not stop the delete",
			att:  models.FileAttachment{FileType: "image/png", S3Key: "users/u/t.png"},
			setup: func(_ *memStore, f *memFiles, _ *memProvider, a models.FileAttachment) {
				f.failOn = map[string]bool{storage.FileKeyForImageThumbnail(user, a.ID): true}
			},
			wantRowGone: true,
			wantKeys:    func(models.FileAttachment) []string { return []string{"users/u/t.png"} },
		},
		{
			name:          "file: provider copy, row, then its unreferenced object",
			att:           models.FileAttachment{FileType: "application/pdf", S3Key: "users/u/doc.pdf", FileID: strPtr("file-1")},
			wantRowGone:   true,
			wantKeys:      func(models.FileAttachment) []string { return []string{"users/u/doc.pdf"} },
			wantProviders: []string{"file-1"},
		},
		{
			name: "file sharing its provider copy and object with a reference copy leaves both",
			att:  models.FileAttachment{FileType: "text/markdown", S3Key: "users/u/n.md", FileID: strPtr("file-2")},
			setup: func(s *memStore, _ *memFiles, _ *memProvider, _ models.FileAttachment) {
				s.add(user, models.FileAttachment{FileType: "text/markdown", S3Key: "users/u/n.md", FileID: strPtr("file-2")})
			},
			wantRowGone: true,
		},
		{
			name: "file whose provider copy cannot be deleted keeps its row",
			att:  models.FileAttachment{FileType: "application/pdf", FileID: strPtr("file-3")},
			setup: func(_ *memStore, _ *memFiles, p *memProvider, _ models.FileAttachment) {
				p.err = errors.New("provider down")
			},
			wantStage: StageProvider,
		},
		{
			name: "a failed row delete is a record failure",
			att:  models.FileAttachment{FileType: "text/plain"},
			setup: func(s *memStore, _ *memFiles, _ *memProvider, a models.FileAttachment) {
				s.deleteErr[a.ID] = errors.New("db down")
			},
			wantStage: StageRecord,
		},
		{
			name: "a failed lookup is a lookup failure",
			att:  models.FileAttachment{FileType: "image/png"},
			setup: func(s *memStore, _ *memFiles, _ *memProvider, a models.FileAttachment) {
				s.getErr[a.ID] = errors.New("db down")
			},
			wantStage: StageLookup,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, files, provider := newMemStore(), &memFiles{}, &memProvider{}
			att := store.add(user, tt.att)
			if tt.setup != nil {
				tt.setup(store, files, provider, att)
			}
			err := New(store, files, provider, zap.NewNop()).Delete(context.Background(), user, att.ID)

			if tt.wantStage != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantStage, StageOf(err))
				assert.True(t, store.has(att.ID), "row should survive a failed delete")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRowGone, !store.has(att.ID))
			var want []string
			if tt.wantKeys != nil {
				want = tt.wantKeys(att)
				sort.Strings(want)
			}
			assert.Equal(t, want, files.deletedKeys())
			assert.Equal(t, tt.wantProviders, provider.deleted)
		})
	}
}

func TestDeleteImageRefusesOtherFilesAndStrangers(t *testing.T) {
	user := uuid.New()
	store := newMemStore()
	doc := store.add(user, models.FileAttachment{FileType: "application/pdf"})
	theirs := store.add(uuid.New(), models.FileAttachment{FileType: "image/png"})
	svc := New(store, &memFiles{}, &memProvider{}, nil)

	assert.ErrorIs(t, svc.DeleteImage(context.Background(), user, doc.ID), ErrNotImage)
	assert.ErrorIs(t, svc.DeleteImage(context.Background(), user, theirs.ID), ErrNotFound)
	assert.ErrorIs(t, svc.DeleteFile(context.Background(), user, uuid.New()), ErrNotFound)
	assert.True(t, store.has(doc.ID))
	assert.True(t, store.has(theirs.ID))
}

func TestDeleteFileWithoutAProviderClientKeepsTheRow(t *testing.T) {
	user := uuid.New()
	store := newMemStore()
	att := store.add(user, models.FileAttachment{FileType: "application/pdf", FileID: strPtr("file-9")})

	err := New(store, nil, nil, nil).DeleteFile(context.Background(), user, att.ID)

	assert.Equal(t, StageProvider, StageOf(err))
	assert.True(t, store.has(att.ID))
}

func TestDeleteManyReportsEachIDAndKeepsGoing(t *testing.T) {
	user := uuid.New()
	store, files, provider := newMemStore(), &memFiles{}, &memProvider{}
	img := store.add(user, models.FileAttachment{FileType: "image/png", S3Key: "users/u/a.png"})
	doc := store.add(user, models.FileAttachment{FileType: "application/pdf", S3Key: "users/u/b.pdf", FileID: strPtr("file-b")})
	broken := store.add(user, models.FileAttachment{FileType: "image/png", S3Key: "users/u/broken.png"})
	files.failOn = map[string]bool{"users/u/broken.png": true}
	theirs := store.add(uuid.New(), models.FileAttachment{FileType: "image/png", S3Key: "users/x/c.png"})
	missing := uuid.New()

	res := New(store, files, provider, zap.NewNop()).DeleteMany(context.Background(), user,
		[]uuid.UUID{img.ID, theirs.ID, doc.ID, img.ID, broken.ID, missing})

	assert.Equal(t, []uuid.UUID{img.ID, doc.ID}, res.Deleted)
	assert.Equal(t, []Failure{
		{ID: theirs.ID, Error: "not found"},
		{ID: broken.ID, Error: "could not delete the stored image"},
		{ID: missing, Error: "not found"},
	}, res.Failed)
	assert.True(t, store.has(theirs.ID), "another user's file is never touched")
	assert.True(t, store.has(broken.ID))
	assert.Equal(t, []string{"file-b"}, provider.deleted)
}

func TestDeleteManyRunsToCompletionAfterTheCallerHangsUp(t *testing.T) {
	user := uuid.New()
	store := newMemStore()
	var ids []uuid.UUID
	for range 20 {
		ids = append(ids, store.add(user, models.FileAttachment{FileType: "text/plain"}).ID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := New(store, nil, nil, nil).DeleteMany(ctx, user, ids)

	assert.Len(t, res.Deleted, 20)
	assert.Empty(t, res.Failed)
}

func TestPublicMessage(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{ErrNotFound, "not found"},
		{ErrNotImage, "not an image"},
		{&StepError{Stage: StageLookup, Err: errors.New("x")}, "could not read the file"},
		{&StepError{Stage: StageStorage, Err: errors.New("x")}, "could not delete the stored image"},
		{&StepError{Stage: StageProvider, Err: errors.New("x")}, "could not delete the provider's copy"},
		{&StepError{Stage: StageRecord, Err: errors.New("x")}, "could not delete the file"},
		{errors.New("anything else"), "could not delete the file"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, PublicMessage(tt.err))
	}
}
