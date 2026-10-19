package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/theimaginaryfoundation/what-iff/internal/models"
	"github.com/theimaginaryfoundation/what-iff/internal/telemetry"
)

const DefaultOpenRouterBaseURL = "https://openrouter.ai/api/v1"

type OpenRouterProvider struct {
	client *openai.Client
	tel    *telemetry.Telemetry
}

// OpenRouterAdapter implements AgentAdapter for OpenRouter's OpenAI-compatible
// Chat Completions API. When a text-delta handler is set the turn streams
// token deltas; otherwise it falls back to a single non-streaming request.
// The full assistant text is always available via GenerateResponse.
type OpenRouterAdapter struct {
	provider         *OpenRouterProvider
	params           openai.ChatCompletionNewParams
	textDeltaHandler func(delta string)
}

func NewOpenRouterProvider(apiKey, baseURL string, tel *telemetry.Telemetry, httpClient *http.Client) *OpenRouterProvider {
	opts := []option.RequestOption{option.WithAPIKey(apiKey), option.WithMaxRetries(2), option.WithHeader("HTTP-Referer", "https://github.com/MiriNex/Whatiff-Matrix"), option.WithHeader("X-Title", "WhatIff")}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultOpenRouterBaseURL
	}
	if strings.TrimSpace(baseURL) != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	client := openai.NewClient(opts...)
	return &OpenRouterProvider{client: &client, tel: tel}
}

func (p *OpenRouterProvider) Call(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	return chatCompletionsNew(ctx, p.tel, telemetry.DependencyOpenRouter, p.client, params)
}

// CallStreaming streams a Chat Completions request, forwarding text deltas to onTextDelta.
func (p *OpenRouterProvider) CallStreaming(ctx context.Context, params openai.ChatCompletionNewParams, onTextDelta func(delta string)) (*openai.ChatCompletion, error) {
	resp, err := chatCompletionsStream(ctx, p.tel, telemetry.DependencyOpenRouter, p.client, params, chatCompletionStreamHooks{onTextDelta: onTextDelta})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func NewOpenRouterAdapter(provider *OpenRouterProvider, params openai.ChatCompletionNewParams, functionTools []openai.ChatCompletionToolUnionParam, disabledTools map[string]bool) *OpenRouterAdapter {
	for _, t := range functionTools {
		if t.OfFunction != nil && disabledTools[t.OfFunction.Function.Name] {
			continue
		}
		params.Tools = append(params.Tools, t)
	}
	return &OpenRouterAdapter{provider: provider, params: params}
}

func (a *OpenRouterAdapter) Call(ctx context.Context) (*GenerateResponse, []ToolUse, error) {
	resp, err := a.call(ctx)
	if err != nil {
		return nil, nil, WrapSafetyViolationError(models.SafetyViolationProviderOpenRouter, fmt.Errorf("OpenRouter API call failed: %w", err))
	}
	if len(resp.Choices) == 0 {
		return nil, nil, fmt.Errorf("OpenRouter API returned no choices")
	}
	toolUses := extractChatCompletionToolUses(resp)
	if len(toolUses) == 0 {
		return a.toGenerateResponse(resp), nil, nil
	}
	a.params.Messages = append(a.params.Messages, chatCompletionAssistantReplay(resp.Choices[0].Message, ""))
	return nil, toolUses, nil
}

func (a *OpenRouterAdapter) AppendToolResults(results []ToolResult) {
	for _, r := range results {
		a.params.Messages = append(a.params.Messages, openai.ToolMessage(toolResultOutput(r), r.ID))
	}
}

func (a *OpenRouterAdapter) ForceFinalResponse(ctx context.Context) (*GenerateResponse, error) {
	a.params.Tools = nil
	a.params.Messages = append(a.params.Messages, openai.UserMessage("Please provide your best final response based on the information gathered so far without additional tool calls."))
	resp, err := a.call(ctx)
	if err != nil {
		return nil, WrapSafetyViolationError(models.SafetyViolationProviderOpenRouter, fmt.Errorf("OpenRouter final-response call failed: %w", err))
	}
	return a.toGenerateResponse(resp), nil
}

// call streams when a text-delta handler is set, else issues a non-streaming request.
func (a *OpenRouterAdapter) call(ctx context.Context) (*openai.ChatCompletion, error) {
	if a.textDeltaHandler != nil {
		return a.provider.CallStreaming(ctx, a.params, a.textDeltaHandler)
	}
	return a.provider.Call(ctx, a.params)
}

func (a *OpenRouterAdapter) WebSearchCompletedCount() int { return 0 }

// SetTextDeltaHandler stores the handler; when set, the OpenRouter path streams token deltas.
func (a *OpenRouterAdapter) SetTextDeltaHandler(handler func(delta string)) {
	a.textDeltaHandler = handler
}

func (a *OpenRouterAdapter) toGenerateResponse(resp *openai.ChatCompletion) *GenerateResponse {
	inputTokens, outputTokens := chatCompletionTokenUsage(resp)
	return &GenerateResponse{
		ID:           resp.ID,
		Text:         ExtractChatCompletionText(resp),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}
}
