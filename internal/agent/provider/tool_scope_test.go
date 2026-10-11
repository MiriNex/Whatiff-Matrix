package provider

import (
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/require"
)

func hideNamed(names ...string) func(string) bool {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return func(name string) bool { return set[name] }
}

func chatCompletionToolNames(tools []openai.ChatCompletionToolUnionParam) []string {
	var out []string
	for _, t := range tools {
		out = append(out, chatCompletionToolName(t))
	}
	return out
}

func TestScopedToolList(t *testing.T) {
	list := newScopedToolList([]string{"a", "", "b", "c"}, func(s string) string { return s })
	require.Equal(t, []string{"a", "", "b", "c"}, list.scope(nil))
	require.Equal(t, []string{"a", "", "c"}, list.scope(hideNamed("b")))
	// Every round chooses from the full list, not from what the last round left.
	require.Equal(t, []string{"a", "", "b", "c"}, list.scope(hideNamed()))
	// A tool with no function name (a vendor-native tool) is always kept.
	require.Equal(t, []string{""}, list.scope(hideNamed("a", "b", "c")))
	require.Nil(t, newScopedToolList([]string{"a"}, func(s string) string { return s }).scope(hideNamed("a")))
}

func TestChatCompletionAdapterScopeTools(t *testing.T) {
	fn := func(name string) openai.ChatCompletionToolUnionParam {
		return OpenAIChatCompletionFunctionTool(name, "d", map[string]any{}, nil)
	}
	adapter := NewLocalAdapter(nil, openai.ChatCompletionNewParams{}, []openai.ChatCompletionToolUnionParam{fn("recall"), fn("web_search"), fn("off")}, map[string]bool{"off": true})
	require.Equal(t, []string{"recall", "web_search"}, chatCompletionToolNames(adapter.params.Tools))
	adapter.ScopeTools(hideNamed("web_search"))
	require.Equal(t, []string{"recall"}, chatCompletionToolNames(adapter.params.Tools))
	adapter.ScopeTools(hideNamed())
	require.Equal(t, []string{"recall", "web_search"}, chatCompletionToolNames(adapter.params.Tools), "a disabled tool never comes back")
}

func TestClaudeAdapterScopeToolsKeepsNativeWebSearch(t *testing.T) {
	tools := []anthropic.ToolUnionParam{
		ClaudeFunctionTool("recall", "d", map[string]any{}, nil, false),
		ClaudeFunctionTool("web_search", "d", map[string]any{}, nil, false),
	}
	adapter := NewClaudeAdapter(nil, anthropic.MessageNewParams{}, tools, true, nil, nil)
	require.Len(t, adapter.params.Tools, 3)
	adapter.ScopeTools(hideNamed("web_search"))
	require.Len(t, adapter.params.Tools, 2)
	require.Equal(t, "recall", claudeToolName(adapter.params.Tools[0]))
	require.Equal(t, "", claudeToolName(adapter.params.Tools[1]), "the vendor's native web search is not a function tool and stays")
}

func TestOpenAIAdapterScopeTools(t *testing.T) {
	params := responses.ResponseNewParams{Tools: []responses.ToolUnionParam{
		{OfWebSearch: &responses.WebSearchToolParam{Type: responses.WebSearchToolTypeWebSearch}},
		{OfFunction: &responses.FunctionToolParam{Name: "recall"}},
		{OfFunction: &responses.FunctionToolParam{Name: "mcp__x__y"}},
	}}
	adapter := NewOpenAIAdapter(nil, params)
	adapter.ScopeTools(hideNamed("mcp__x__y"))
	got := adapter.CurrentParams().Tools
	require.Len(t, got, 2)
	require.NotNil(t, got[0].OfWebSearch)
	require.Equal(t, "recall", got[1].OfFunction.Name)
}
