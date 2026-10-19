package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"go.uber.org/zap"

	"github.com/theimaginaryfoundation/what-iff/internal/telemetry"
)

// utilityLLMClient returns an OpenAI-compatible client for internal utility
// calls (memory query, chat naming) backed by the configured utility LLM
// (e.g. Gemini's OpenAI-compatible endpoint). Returns nil when unconfigured.
func utilityLLMClient() *openai.Client {
	baseURL := strings.TrimSpace(os.Getenv("UTIL_LLM_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("UTIL_LLM_API_KEY"))
	if baseURL == "" || apiKey == "" {
		return nil
	}
	client := openai.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey(apiKey),
	)
	return &client
}

func utilityLLMModel() string {
	if m := strings.TrimSpace(os.Getenv("UTIL_LLM_MODEL")); m != "" {
		return m
	}
	return "gemini-3.5-flash"
}

// utilityChatCompletion runs a small system+user chat completion on the
// primary utility LLM, falling back to the utility chain (OpenRouter free
// models, then z.ai GLM) when the primary is unconfigured or fails.
func (a *Agent) utilityChatCompletion(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	if a.testHooks.UtilityChatCompletion != nil {
		return a.testHooks.UtilityChatCompletion(ctx, systemPrompt, userMessage)
	}
	client := utilityLLMClient()
	if client != nil {
		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: utilityLLMModel(),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(systemPrompt),
				openai.UserMessage(userMessage),
			},
		})
		if err != nil {
			a.logger.Warn("utility LLM primary failed; walking utility chain fallbacks", zap.Error(err))
		} else if len(resp.Choices) == 0 {
			a.logger.Warn("utility LLM returned no choices; walking utility chain fallbacks")
		} else if text := strings.TrimSpace(resp.Choices[0].Message.Content); text != "" {
			return text, nil
		}
	}
	text, err := a.utilityChainFallbacks(ctx, telemetry.CallPathUnknown, systemPrompt, userMessage, 0)
	if err != nil {
		return "", fmt.Errorf("utility LLM failed: %w", err)
	}
	return text, nil
}

// stripJSONFences removes markdown code fences that chat-completions models
// sometimes wrap around JSON replies.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSpace(s)
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
