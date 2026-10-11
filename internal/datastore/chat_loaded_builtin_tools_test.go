package datastore

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestChatLoadedBuiltinTools_SeparateFromConnectors(t *testing.T) {
	ds, cleanup := newMCPServerTestDatastore(t)
	defer cleanup()
	ctx := context.Background()

	userID := createMCPServerTestUser(t, ds)
	chatID := createMCPServerTestChat(t, ds, userID, createMCPServerTestModel(t, ds))
	server, err := ds.CreateMCPServer(ctx, userID, baseMCPServerModel())
	require.NoError(t, err)
	require.NoError(t, ds.SetChatMCPLoadedTools(ctx, userID, chatID, server.ID, []string{"mcp__server__search"}))

	require.NoError(t, ds.SetChatLoadedBuiltinTools(ctx, userID, chatID, []string{"web_search", " generate_image ", "web_search", "mcp__server__search", ""}))
	builtin, err := ds.ListChatLoadedBuiltinTools(ctx, userID, chatID)
	require.NoError(t, err)
	require.Equal(t, []string{"generate_image", "web_search"}, builtin)

	byServer, err := ds.ListChatMCPLoadedTools(ctx, userID, chatID)
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID][]string{server.ID: {"mcp__server__search"}}, byServer, "the built-in row is not listed as a connector")

	require.ErrorIs(t, ds.SetChatMCPLoadedTools(ctx, userID, chatID, uuid.Nil, []string{"mcp__x__y"}), ErrMCPServerNotFound)

	require.NoError(t, ds.SetChatLoadedBuiltinTools(ctx, userID, chatID, nil))
	builtin, err = ds.ListChatLoadedBuiltinTools(ctx, userID, chatID)
	require.NoError(t, err)
	require.Empty(t, builtin)

	_, err = ds.ListChatLoadedBuiltinTools(ctx, uuid.New(), chatID)
	require.ErrorIs(t, err, ErrChatNotFound)
}
