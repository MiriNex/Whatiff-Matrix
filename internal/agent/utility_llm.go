package agent

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
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
// utility LLM and returns the text reply.
func (a *Agent) utilityChatCompletion(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	client := utilityLLMClient()
	if client == nil {
		return "", errors.New("utility LLM not configured (set UTIL_LLM_BASE_URL and UTIL_LLM_API_KEY)")
	}
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: utilityLLMModel(),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userMessage),
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("utility LLM returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
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
