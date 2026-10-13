package agent

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/zap"
)

// chatNameModel is the legacy OpenAI Responses model name still referenced by
// expression generation utilities (expression_pick, likeness). TODO: migrate those
// to the utility LLM (utilityChatCompletion) alongside memory query and chat naming.
const chatNameModel = "gpt-4.1-nano-2025-04-14"

func (a *Agent) generateChatName(ctx context.Context, userMessage string) (string, error) {
	// Mock/local mode: deterministic fake — no provider call.
	if a.nonVendorLLM() {
		return mockChatName(userMessage), nil
	}
	name, err := a.utilityChatCompletion(ctx, chatNamePrompt, userMessage)
	if err != nil {
		a.logger.Error("failed to generate chat name", zap.Error(err))
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("utility LLM returned empty chat name")
	}
	return name, nil
}

// mockChatName derives a deterministic chat name from the first words of the
// user message, used under MOCK_LLM instead of the naming model.
func mockChatName(userMessage string) string {
	const maxLen = 40
	name := strings.Join(strings.Fields(userMessage), " ")
	if name == "" {
		return "Mock Chat"
	}
	runes := []rune(name)
	if len(runes) > maxLen {
		name = string(runes[:maxLen]) + "…"
	}
	return name
}
