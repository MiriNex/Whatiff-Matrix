package imagegallery

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/theimaginaryfoundation/what-iff/internal/handlers/handlerutils"
	"github.com/theimaginaryfoundation/what-iff/internal/middleware"
)

type deleteFilesRequest struct {
	IDs []uuid.UUID `json:"ids"`
}

// DeleteFiles deletes many gallery files at once, each with the cleanup its own route does
// (images as DELETE /image-gallery/{id}, other files as DELETE /file-attachment/{id}). It answers
// 200 with which ids were deleted and which failed and why; ids that are not the caller's files
// fail as "not found" rather than failing the batch.
func (h *Handler) DeleteFiles(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		handlerutils.RespondWithError(w, h.logger, http.StatusUnauthorized, handlerutils.CodeNotSet, "Unauthorized", nil)
		return
	}
	var req deleteFilesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBatchBodyLen)).Decode(&req); err != nil {
		handlerutils.RespondWithError(w, h.logger, http.StatusBadRequest, handlerutils.CodeNotSet, "Invalid request body", err)
		return
	}
	if len(req.IDs) == 0 {
		handlerutils.RespondWithError(w, h.logger, http.StatusBadRequest, handlerutils.CodeNotSet, "Choose at least one file", nil)
		return
	}
	if len(req.IDs) > maxBatchIDs {
		handlerutils.RespondWithError(w, h.logger, http.StatusBadRequest, handlerutils.CodeNotSet, "Too many files in one delete", nil)
		return
	}
	handlerutils.RespondWithJSON(w, h.logger, http.StatusOK, h.deleter().DeleteMany(r.Context(), userID, req.IDs))
}
