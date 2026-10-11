package provider

import (
	"slices"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

// ToolScoper is implemented by adapters whose function tools can be narrowed between rounds.
// The agent loop calls ScopeTools before each Call with the tools to leave out of that request:
// the loadable ones (MCP tools, and the heavy built-in tools) that are not loaded in the chat.
// A tool loaded mid-turn is then sent from the next round on, and a tool that is never loaded
// never costs its definition's tokens. Tools that are not function tools, such as a vendor's
// native web search, are always kept.
type ToolScoper interface {
	ScopeTools(hidden func(name string) bool)
}

// scopedToolList keeps every tool an adapter was built with, so each round can choose from the
// full set rather than from what an earlier round left.
type scopedToolList[T any] struct {
	all []T
	// name returns a function tool's name, or "" for any other tool (which is always kept).
	name func(T) string
}

func newScopedToolList[T any](tools []T, name func(T) string) scopedToolList[T] {
	return scopedToolList[T]{all: slices.Clone(tools), name: name}
}

// scope returns the tools hidden does not exclude, in their original order. A nil hidden keeps
// every tool. An empty result is nil so the request omits the tools field.
func (l scopedToolList[T]) scope(hidden func(name string) bool) []T {
	out := make([]T, 0, len(l.all))
	for _, t := range l.all {
		if n := l.name(t); n != "" && hidden != nil && hidden(n) {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func chatCompletionToolName(t openai.ChatCompletionToolUnionParam) string {
	if t.OfFunction != nil {
		return t.OfFunction.Function.Name
	}
	return ""
}

func responsesToolName(t responses.ToolUnionParam) string {
	if t.OfFunction != nil {
		return t.OfFunction.Name
	}
	return ""
}

func claudeScopedTools(tools []anthropic.ToolUnionParam) scopedToolList[anthropic.ToolUnionParam] {
	return newScopedToolList(tools, claudeToolName)
}

// chatCompletionScopedTools appends functionTools minus the disabled ones to params.Tools (what
// every Chat Completions adapter is built with) and returns the list ScopeTools picks from.
func chatCompletionScopedTools(params *openai.ChatCompletionNewParams, functionTools []openai.ChatCompletionToolUnionParam, disabledTools map[string]bool) scopedToolList[openai.ChatCompletionToolUnionParam] {
	for _, t := range functionTools {
		if t.OfFunction != nil && disabledTools[t.OfFunction.Function.Name] {
			continue
		}
		params.Tools = append(params.Tools, t)
	}
	return newScopedToolList(params.Tools, chatCompletionToolName)
}
