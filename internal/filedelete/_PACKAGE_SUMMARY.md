# Package: `internal/filedelete`

## Role

Deletes a user's file attachments with the cleanup each kind needs.
It is the one implementation behind `DELETE /image-gallery/{id}`, `DELETE /file-attachment/{id}` and the gallery's bulk `POST /image-gallery/delete`.

## Responsibilities

- **`Service.DeleteImage`:** images only (`ErrNotImage` otherwise).
  Deletes the full-resolution object first and stops if that fails, so the row and storage do not drift.
  The thumbnail delete is best effort; then the row goes.
- **`Service.DeleteFile`:** any attachment.
  Deletes the provider copy unless another row (a reference copy) shares its `FileID`, then the row, then releases the stored objects through `storage.ReleaseAttachmentObjects` (best effort, with `context.WithoutCancel`).
- **`Service.Delete`:** one gallery file as the web app's single delete does for its kind: images as `DeleteImage`, everything else as `DeleteFile`.
- **`Service.DeleteMany`:** `Delete` for each id, eight at a time, deduplicated, in input order.
  Returns `BulkResult{Deleted, Failed}`; one failure never stops the rest.
  It runs to completion even if the request is cancelled.
- **Errors:** `ErrNotFound` (not the caller's row, or missing), `ErrNotImage`, and `*StepError` naming the `Stage` that failed (`lookup`, `storage`, `provider`, `record`).
  `StageOf` lets each route keep its own statuses; `PublicMessage` is the user-safe text in a bulk failure.

## Dependencies

- **Inbound:** `internal/handlers/imagegallery`, `internal/handlers/fileattachment`.
- **Outbound:** `internal/datastore` (`ErrFileAttachmentNotFound`), `internal/storage`, `internal/models`, `golang.org/x/sync/errgroup`.
- **`Store`** is the narrow datastore surface (get, delete, provider-file sharing check, `storage.AttachmentKeyRefs`); `*datastore.Datastore` satisfies it.
- **`ProviderFiles`** deletes a provider copy; `*agent.Agent` satisfies it.

## Non-obvious decisions

- **The two kinds keep their historical cleanup.**
  The image path deletes the stored object directly and fails fast; the file path is reference counted and best effort.
  The bulk route reproduces exactly what the client did before it existed (one route per kind), so moving to it changed no cleanup.
- **Ownership is the datastore's.**
  Every lookup and delete is scoped to the caller, so an id that is someone else's is `ErrNotFound`, reported per id in a bulk delete rather than failing the batch.
- **No provider client means no delete** for a file with a provider copy (`StageProvider`), rather than orphaning the copy.
- **Bulk deletes reuse the single-row datastore calls** rather than a batch query: each row needs its own storage and provider steps anyway, and the route caps a batch at 500 ids.

## Testing

- `filedelete_test.go` — table-driven per-kind cleanup (keys, derived legacy key, thumbnail best effort, provider copy, reference-shared copy kept) and the failing stage of each step.
  Also `DeleteImage` refusals, a missing provider client, `DeleteMany` (mixed kinds, strangers' and missing ids, duplicates, a failure mid-batch, order, cancelled context) and `PublicMessage`.
- Route-level behavior is tested in `internal/handlers/imagegallery/delete_test.go` and `internal/handlers/fileattachment`.

## Related documentation

- [Architecture summary](../../docs/ARCHITECTURE_SUMMARY.md) — attachments and S3.
- `internal/storage/_PACKAGE_SUMMARY.md` — `ReleaseAttachmentObjects`.
