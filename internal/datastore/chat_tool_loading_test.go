package datastore

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/theimaginaryfoundation/what-iff/ent/chatmessage"
)

func TestChatToolNamesUsedSinceAndRetainChatLoadedTools(t *testing.T) {
	ds, cleanup := newMCPServerTestDatastore(t)
	defer cleanup()
	ctx := context.Background()

	userID := createMCPServerTestUser(t, ds)
	modelID := createMCPServerTestModel(t, ds)
	chatID := createMCPServerTestChat(t, ds, userID, modelID)
	otherChatID := createMCPServerTestChat(t, ds, userID, modelID)
	server, err := ds.CreateMCPServer(ctx, userID, baseMCPServerModel())
	require.NoError(t, err)

	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	message := func(chat uuid.UUID, at time.Time, tools ...string) {
		msg, err := ds.dbClient.ChatMessage.Create().
			SetChatID(chat).
			SetMessage("m").
			SetOrigin(chatmessage.OriginAssistant).
			SetSentAt(at).
			Save(ctx)
		require.NoError(t, err)
		for _, name := range tools {
			require.NoError(t, ds.dbClient.ToolCall.Create().SetChatMessageID(msg.ID).SetToolName(name).Exec(ctx))
		}
	}
	message(chatID, base, "generate_image")                                   // before the segment
	message(chatID, base.Add(10*time.Minute), "web_search", "mcp__server__a") // in the segment
	message(chatID, base.Add(20*time.Minute), "web_search")
	message(otherChatID, base.Add(20*time.Minute), "run_subagent")

	segmentStart := base.Add(5 * time.Minute)
	used, err := ds.ChatToolNamesUsedSince(ctx, userID, chatID, &segmentStart)
	require.NoError(t, err)
	require.Equal(t, map[string]struct{}{"web_search": {}, "mcp__server__a": {}}, used)

	all, err := ds.ChatToolNamesUsedSince(ctx, userID, chatID, nil)
	require.NoError(t, err)
	require.Contains(t, all, "generate_image")
	require.NotContains(t, all, "run_subagent")

	require.NoError(t, ds.SetChatMCPLoadedTools(ctx, userID, chatID, server.ID, []string{"mcp__server__a", "mcp__server__b"}))
	require.NoError(t, ds.SetChatLoadedBuiltinTools(ctx, userID, chatID, []string{"generate_image", "web_search"}))

	unloaded, err := ds.RetainChatLoadedTools(ctx, userID, chatID, used)
	require.NoError(t, err)
	require.Equal(t, []string{"generate_image", "mcp__server__b"}, unloaded)

	builtin, err := ds.ListChatLoadedBuiltinTools(ctx, userID, chatID)
	require.NoError(t, err)
	require.Equal(t, []string{"web_search"}, builtin)
	byServer, err := ds.ListChatMCPLoadedTools(ctx, userID, chatID)
	require.NoError(t, err)
	require.Equal(t, []string{"mcp__server__a"}, byServer[server.ID])

	// Nothing used: every row is cleared.
	unloaded, err = ds.RetainChatLoadedTools(ctx, userID, chatID, map[string]struct{}{})
	require.NoError(t, err)
	require.Equal(t, []string{"mcp__server__a", "web_search"}, unloaded)
	byServer, err = ds.ListChatMCPLoadedTools(ctx, userID, chatID)
	require.NoError(t, err)
	require.Empty(t, byServer)

	_, err = ds.RetainChatLoadedTools(ctx, uuid.New(), chatID, nil)
	require.ErrorIs(t, err, ErrChatNotFound)
}
