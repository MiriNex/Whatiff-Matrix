package filedelete

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// bulkConcurrency is how many deletes DeleteMany runs at once: each is a few storage and database
// round trips, so a full batch finishes in reasonable time without crowding the pool.
const bulkConcurrency = 8

// Failure is one id DeleteMany could not delete, with a message safe to show the user.
type Failure struct {
	ID    uuid.UUID `json:"id"`
	Error string    `json:"error"`
}

// BulkResult says what DeleteMany did with each id: deleted, or failed and why.
type BulkResult struct {
	Deleted []uuid.UUID `json:"deleted"`
	Failed  []Failure   `json:"failed"`
}

// DeleteMany deletes each of ids as Delete would (by kind), a few at a time. One that fails does
// not stop the rest: ids that are not the caller's files (or do not exist) fail as not found, and
// others fail with the step that went wrong. Duplicate ids are deleted once. Results keep the
// order of ids.
//
// It runs to completion even if ctx is cancelled (the client hung up), so a batch is never left
// half done without anyone knowing which half.
func (s *Service) DeleteMany(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) BulkResult {
	ctx = context.WithoutCancel(ctx)
	unique := dedupe(ids)
	errs := make([]error, len(unique))
	var g errgroup.Group
	g.SetLimit(bulkConcurrency)
	for i, id := range unique {
		g.Go(func() error {
			errs[i] = s.Delete(ctx, userID, id)
			return nil
		})
	}
	_ = g.Wait()

	res := BulkResult{Deleted: []uuid.UUID{}, Failed: []Failure{}}
	for i, id := range unique {
		if errs[i] == nil {
			res.Deleted = append(res.Deleted, id)
			continue
		}
		if !errors.Is(errs[i], ErrNotFound) {
			s.logger.Warn("file delete: bulk delete item failed",
				zap.String("user_id", userID.String()), zap.String("id", id.String()), zap.Error(errs[i]))
		}
		res.Failed = append(res.Failed, Failure{ID: id, Error: PublicMessage(errs[i])})
	}
	return res
}

// PublicMessage is what a user is told about a failed delete: what failed, not the internals.
func PublicMessage(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNotFound):
		return "not found"
	case errors.Is(err, ErrNotImage):
		return "not an image"
	}
	switch StageOf(err) {
	case StageLookup:
		return "could not read the file"
	case StageStorage:
		return "could not delete the stored image"
	case StageProvider:
		return "could not delete the provider's copy"
	default:
		return "could not delete the file"
	}
}

func dedupe(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
