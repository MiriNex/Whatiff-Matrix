package datastore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theimaginaryfoundation/what-iff/ent/embedding"
	"github.com/theimaginaryfoundation/what-iff/ent/migrate"
)

// Postgres does not index a foreign key by itself. Without an index on embeddings.embedding_memory
// every lookup by memory (the missing-embedding NOT EXISTS checks and backfill pages, replacing or
// deleting a memory's embedding, and the FK check when a memory is deleted) scans the whole table,
// whose rows carry a vector(1536). This is schema-level, so it needs no database: it fails if the
// index ever drops out of the Ent schema (#266).
func TestEmbeddingMemoryForeignKeyIsIndexed(t *testing.T) {
	require.Equal(t, "embedding_memory", embedding.MemoryColumn, "the FK column this test guards has been renamed; update it")

	var found bool
	for _, idx := range migrate.EmbeddingsTable.Indexes {
		if len(idx.Columns) > 0 && idx.Columns[0].Name == embedding.MemoryColumn {
			found = true
			require.False(t, idx.Unique, "writers delete-then-insert and other paths may briefly coexist, so the index must not be unique")
		}
	}
	require.True(t, found, "embeddings.%s must lead an index so lookups by memory do not scan the table", embedding.MemoryColumn)
}
