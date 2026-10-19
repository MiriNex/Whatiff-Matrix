package agent

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"go.uber.org/zap"

	"github.com/theimaginaryfoundation/what-iff/internal/agent/provider"
	"github.com/theimaginaryfoundation/what-iff/internal/telemetry"
)

// Utility fallback chain for the small internal LLM calls (chat naming, memory
// query, expression picking, personality generation). Order, fixed 2026-10-19
// for the self-hosted MiriNex fork: the configured primary utility LLM
// (UTIL_LLM_*) first, then OpenRouter free models, and the z.ai GLM key only
// as a last resort so its paid credits are not burned first.

// utilityChainZAIModel is the GLM model used for the chain's last-resort step.
const utilityChainZAIModel = "glm-5.3"

// utilityChainDefaultZAIMaxTokens caps the last-resort z.ai step when callers
// leave maxTokens unset (chat-completions steps have no cap in that case).
const utilityChainDefaultZAIMaxTokens = 2048

// defaultOpenRouterUtilityModels are tried (in order) when the primary utility
// LLM fails: Nemotron 3.5 Lightning first (task specialist, fast, verified
// live), then the free auto-router. Gemma free is excluded: its shared upstream
// pool is permanently rate-limited. LFM free suffers the same and stays out.
const defaultOpenRouterUtilityModels = "nvidia/nemotron-3.5-lightning:free,openrouter/free"

// utilityChainCodestralModel is the Mistral step of the chain: codestral is
// verified live on the free Mistral tier (mistral-small/medium answer 429).
const utilityChainCodestralModel = "codestral-latest"

// utilityChainGeminiModel is the direct-Gemini step of the chain, tried when
// OpenRouter and Codestral both fail. It uses GEMINI_API_KEY directly (not the
// UTIL_LLM_* primary, which may already have failed).
const utilityChainGeminiModel = "gemini-3.5-flash"

// utilityChainGeminiBaseURL is Gemini's OpenAI-compatible endpoint.
const utilityChainGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"

// openRouterUtilityModels returns the OpenRouter model ids tried for utility
// calls, from OPENROUTER_UTILITY_MODELS (comma-separated) or the default list.
func openRouterUtilityModels() []string {
	raw := strings.TrimSpace(os.Getenv("OPENROUTER_UTILITY_MODELS"))
	if raw == "" {
		raw = defaultOpenRouterUtilityModels
	}
	var modelIDs []string
	for _, m := range strings.Split(raw, ",") {
		if m = strings.TrimSpace(m); m != "" {
			modelIDs = append(modelIDs, m)
		}
	}
	return modelIDs
}

// utilityChainMessages builds the system+user message list shared by the
// chat-completions chain steps.
func utilityChainMessages(systemPrompt, userMessage string) []openai.ChatCompletionMessageParamUnion {
	return []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userMessage),
	}
}

// utilityChainCapTokens applies the optional output token cap to chat
// completions params (maxTokens <= 0 leaves the provider default).
func utilityChainCapTokens(params *openai.ChatCompletionNewParams, maxTokens int) {
	if maxTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(maxTokens))
	}
}

// utilityChainGeminiClient returns an OpenAI-compatible client pointed at
// Gemini's OpenAI endpoint using GEMINI_API_KEY directly. Used by the chain's
// Gemini step when the primary utility LLM (UTIL_LLM_*) has already failed.
// Returns nil when GEMINI_API_KEY is unset.
func utilityChainGeminiClient() *openai.Client {
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		return nil
	}
	client := openai.NewClient(
		option.WithBaseURL(utilityChainGeminiBaseURL),
		option.WithAPIKey(apiKey),
	)
	return &client
}

// utilityChainChat runs a plain system+user chat through the full fallback
// chain — primary utility LLM, OpenRouter models, then z.ai GLM — and returns
// the first non-empty reply.
func (a *Agent) utilityChainChat(ctx context.Context, callPath telemetry.CallPath, systemPrompt, userMessage string, maxTokens int) (string, error) {
	if client := utilityLLMClient(); client != nil {
		params := openai.ChatCompletionNewParams{
			Model:    utilityLLMModel(),
			Messages: utilityChainMessages(systemPrompt, userMessage),
		}
		utilityChainCapTokens(&params, maxTokens)
		resp, err := client.Chat.Completions.New(telemetry.WithCallPath(ctx, callPath), params)
		if err != nil {
			a.logger.Warn("utility chain: primary utility LLM failed; trying OpenRouter", zap.Error(err))
		} else if len(resp.Choices) > 0 {
			if text := strings.TrimSpace(resp.Choices[0].Message.Content); text != "" {
				return text, nil
			}
		}
	}
	return a.utilityChainFallbacks(ctx, callPath, systemPrompt, userMessage, maxTokens)
}

// utilityChainFallbacks runs only the post-primary chain steps (OpenRouter
// models, Codestral, Gemini flash, then z.ai GLM). Use when the primary utility
// LLM was already tried or is not configured.
func (a *Agent) utilityChainFallbacks(ctx context.Context, callPath telemetry.CallPath, systemPrompt, userMessage string, maxTokens int) (string, error) {
	if a.OpenRouterProvider != nil {
		for _, model := range openRouterUtilityModels() {
			params := openai.ChatCompletionNewParams{
				Model:    model,
				Messages: utilityChainMessages(systemPrompt, userMessage),
			}
			utilityChainCapTokens(&params, maxTokens)
			resp, err := a.OpenRouterProvider.Call(telemetry.WithCallPath(ctx, callPath), params)
			if err != nil {
				a.logger.Warn("utility chain: OpenRouter model failed", zap.String("model", model), zap.Error(err))
				continue
			}
			if text := strings.TrimSpace(provider.ExtractChatCompletionText(resp)); text != "" {
				return text, nil
			}
		}
	}
	if a.MistralProvider != nil {
		params := openai.ChatCompletionNewParams{
			Model:    utilityChainCodestralModel,
			Messages: utilityChainMessages(systemPrompt, userMessage),
		}
		utilityChainCapTokens(&params, maxTokens)
		resp, err := a.MistralProvider.Call(telemetry.WithCallPath(ctx, callPath), params)
		if err != nil {
			a.logger.Warn("utility chain: Codestral step failed", zap.Error(err))
		} else if text := strings.TrimSpace(provider.ExtractChatCompletionText(resp)); text != "" {
			return text, nil
		}
	}
	if client := utilityChainGeminiClient(); client != nil {
		params := openai.ChatCompletionNewParams{
			Model:    utilityChainGeminiModel,
			Messages: utilityChainMessages(systemPrompt, userMessage),
		}
		utilityChainCapTokens(&params, maxTokens)
		resp, err := client.Chat.Completions.New(telemetry.WithCallPath(ctx, callPath), params)
		if err != nil {
			a.logger.Warn("utility chain: Gemini flash step failed", zap.Error(err))
		} else if text := strings.TrimSpace(provider.ExtractChatCompletionText(resp)); text != "" {
			return text, nil
		}
	}
	if a.ZAIProvider != nil {
		mc := &provider.ModelContext{}
		mc.Append(provider.SegmentKindSystemPrompt, provider.RoleDeveloper, systemPrompt, true)
		mc.AppendUserMessage(provider.RoleUser, userMessage, nil, false)
		zaiMaxTokens := maxTokens
		if zaiMaxTokens <= 0 {
			zaiMaxTokens = utilityChainDefaultZAIMaxTokens
		}
		params := mc.BuildClaudeParamsWithMaxTokens(utilityChainZAIModel, zaiMaxTokens)
		msg, err := a.ZAIProvider.Call(telemetry.WithCallPath(ctx, callPath), params)
		if err != nil {
			a.logger.Warn("utility chain: z.ai GLM fallback failed", zap.Error(err))
			return "", err
		}
		if text := strings.TrimSpace(provider.ExtractClaudeText(msg)); text != "" {
			return text, nil
		}
	}
	return "", errors.New("utility chain: all providers failed or unconfigured")
}

// utilityChainForkChat runs a full ModelContext fork through the fallback
// chain — primary utility LLM, OpenRouter models, then z.ai GLM — and returns
// the first non-empty reply.
func (a *Agent) utilityChainForkChat(ctx context.Context, callPath telemetry.CallPath, fork *provider.ModelContext, maxTokens int) (string, error) {
	if client := utilityLLMClient(); client != nil {
		params := fork.BuildOpenAIChatCompletionParams(utilityLLMModel())
		utilityChainCapTokens(&params, maxTokens)
		resp, err := client.Chat.Completions.New(telemetry.WithCallPath(ctx, callPath), params)
		if err != nil {
			a.logger.Warn("utility chain: primary utility LLM failed; trying OpenRouter", zap.Error(err))
		} else if text := strings.TrimSpace(provider.ExtractChatCompletionText(resp)); text != "" {
			return text, nil
		}
	}
	if a.OpenRouterProvider != nil {
		for _, model := range openRouterUtilityModels() {
			params := fork.BuildOpenAIChatCompletionParams(model)
			utilityChainCapTokens(&params, maxTokens)
			resp, err := a.OpenRouterProvider.Call(telemetry.WithCallPath(ctx, callPath), params)
			if err != nil {
				a.logger.Warn("utility chain: OpenRouter model failed", zap.String("model", model), zap.Error(err))
				continue
			}
			if text := strings.TrimSpace(provider.ExtractChatCompletionText(resp)); text != "" {
				return text, nil
			}
		}
	}
	if a.MistralProvider != nil {
		params := fork.BuildOpenAIChatCompletionParams(utilityChainCodestralModel)
		utilityChainCapTokens(&params, maxTokens)
		resp, err := a.MistralProvider.Call(telemetry.WithCallPath(ctx, callPath), params)
		if err != nil {
			a.logger.Warn("utility chain: Codestral step failed", zap.Error(err))
		} else if text := strings.TrimSpace(provider.ExtractChatCompletionText(resp)); text != "" {
			return text, nil
		}
	}
	if client := utilityChainGeminiClient(); client != nil {
		params := fork.BuildOpenAIChatCompletionParams(utilityChainGeminiModel)
		utilityChainCapTokens(&params, maxTokens)
		resp, err := client.Chat.Completions.New(telemetry.WithCallPath(ctx, callPath), params)
		if err != nil {
			a.logger.Warn("utility chain: Gemini flash step failed", zap.Error(err))
		} else if text := strings.TrimSpace(provider.ExtractChatCompletionText(resp)); text != "" {
			return text, nil
		}
	}
	if a.ZAIProvider != nil {
		zaiMaxTokens := maxTokens
		if zaiMaxTokens <= 0 {
			zaiMaxTokens = utilityChainDefaultZAIMaxTokens
		}
		params := fork.BuildClaudeParamsWithMaxTokens(utilityChainZAIModel, zaiMaxTokens)
		msg, err := a.ZAIProvider.Call(telemetry.WithCallPath(ctx, callPath), params)
		if err != nil {
			a.logger.Warn("utility chain: z.ai GLM fallback failed", zap.Error(err))
			return "", err
		}
		if text := strings.TrimSpace(provider.ExtractClaudeText(msg)); text != "" {
			return text, nil
		}
	}
	return "", errors.New("utility chain: all providers failed or unconfigured")
}
