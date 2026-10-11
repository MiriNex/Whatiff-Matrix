package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3/responses"
	agenttools "github.com/theimaginaryfoundation/what-iff/internal/agent/tools"
	"go.uber.org/zap"
)

// Loadable tools (#305): MCP tools and the heavy built-in tools the catalog marks Loadable are
// registered with the turn's adapter like any other tool, but the agent loop leaves each one out
// of a request until it is loaded in the chat. Loading persists per chat (chat_mcp_tool_states,
// with the built-in tools under their own row), so a loaded tool stays until unload_tools removes
// it. Because the adapter holds the full set, a tool loaded mid-turn is sent from the next round.

// turnFunctionToolSpecs returns the function tools a chat turn registers with its adapter: the
// agent tools, plus the in-scope MCP tools when tools are enabled. It records the offered set
// (what dispatch accepts) and the chat's load state on chatCtx, so the loop can hide what is
// not loaded.
func (a *Agent) turnFunctionToolSpecs(ctx context.Context, chatCtx *chatContext, userID, chatID uuid.UUID, policy turnToolPolicy) []agenttools.FunctionToolSpec {
	specs := agenttools.AgentFunctionToolSpecs(policy.showMoodTools)
	if policy.toolsEnabled {
		specs = append(specs, a.prepareTurnMCPToolSpecs(ctx, chatCtx, userID, chatID, policy.ritualIDs)...)
	}
	offered := offeredToolNames(specs, policy.disabledTools)
	chatCtx.setOfferedTools(offered)
	a.prepareTurnLoadableTools(ctx, chatCtx, offered)
	return specs
}

// prepareTurnLoadableTools turns on load gating for a chat turn: the offered Loadable built-in
// tools become loadableTools, and the chat's loaded built-in tools are read. A failed read is
// logged and leaves none loaded, so those tools can still be loaded again.
func (a *Agent) prepareTurnLoadableTools(ctx context.Context, chatCtx *chatContext, offered map[string]struct{}) {
	if chatCtx == nil || chatCtx.chat == nil {
		return
	}
	chatCtx.toolsLoadGated = true
	chatCtx.loadableTools = map[string]struct{}{}
	for name := range agenttools.LoadableFunctionToolNames() {
		if _, ok := offered[name]; ok {
			chatCtx.loadableTools[name] = struct{}{}
		}
	}
	chatCtx.loadedTools = map[string]struct{}{}
	if len(chatCtx.loadableTools) == 0 || a.ds == nil {
		return
	}
	loaded, err := a.ds.ListChatLoadedBuiltinTools(ctx, chatCtx.userID, chatCtx.chat.ID)
	if err != nil {
		a.logger.Warn("failed to read the chat's loaded built-in tools",
			zap.String("chat_id", chatCtx.chat.ID.String()),
			zap.Error(err))
		return
	}
	chatCtx.loadedTools = sliceToSet(loaded)
}

// loadedToolFilter returns the predicate the agent loop scopes each request with (see
// provider.ToolScoper): it hides MCP tools and loadable built-in tools that are not loaded. Nil
// when the context is not load-gated (a sub-agent loop, whose MCP tools come from its skills and
// are all offered).
func (c *chatContext) loadedToolFilter() func(name string) bool {
	if c == nil || !c.toolsLoadGated {
		return nil
	}
	return func(name string) bool {
		if strings.HasPrefix(name, "mcp__") {
			return !mcpToolIsLoaded(c, name)
		}
		return c.builtinToolNeedsLoad(name)
	}
}

// builtinToolNeedsLoad reports whether name is a loadable built-in tool that is not loaded.
func (c *chatContext) builtinToolNeedsLoad(name string) bool {
	if c == nil || !c.toolsLoadGated {
		return false
	}
	if _, ok := c.loadableTools[name]; !ok {
		return false
	}
	_, loaded := c.loadedTools[name]
	return !loaded
}

// visibleToolSpecs is specs minus what loadedToolFilter hides: the tools the first request
// actually sends, for the tool-definition token estimate.
func (c *chatContext) visibleToolSpecs(specs []agenttools.FunctionToolSpec) []agenttools.FunctionToolSpec {
	hidden := c.loadedToolFilter()
	if hidden == nil {
		return specs
	}
	out := make([]agenttools.FunctionToolSpec, 0, len(specs))
	for _, spec := range specs {
		if !hidden(spec.Name) {
			out = append(out, spec)
		}
	}
	return out
}

// visibleResponsesTools is toolParams minus the function tools hidden hides (all of them when
// hidden is nil), for the Responses path's tool-definition token estimate.
func visibleResponsesTools(toolParams []responses.ToolUnionParam, hidden func(string) bool) []responses.ToolUnionParam {
	if hidden == nil {
		return toolParams
	}
	out := make([]responses.ToolUnionParam, 0, len(toolParams))
	for _, t := range toolParams {
		if t.OfFunction != nil && hidden(t.OfFunction.Name) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// loadableToolsDeveloperContext names the loadable built-in tools a turn offers, so the model
// knows what it can load without being sent their schemas. Empty when there are none.
func loadableToolsDeveloperContext(offered map[string]struct{}) string {
	summaries := agenttools.LoadableFunctionToolNames()
	names := make([]string, 0, len(summaries))
	for name := range summaries {
		if _, ok := offered[name]; ok {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("Loadable built-in tools: these are not in your toolset until you load them. When a request needs one, call load_tools with its name (no mcp_server_id) and then use it in the same turn:")
	for _, name := range names {
		fmt.Fprintf(&b, "\n- %s: %s", name, summaries[name])
	}
	return b.String()
}

// loadBuiltinTools is load_tools without an mcp_server_id: it loads the named loadable built-in
// tools this turn offers.
func (a *Agent) loadBuiltinTools(ctx context.Context, chatCtx *chatContext, requested requestedTools) (string, error) {
	targets, unknown := resolveRequestedBuiltinTools(chatCtx, requested)
	if len(targets) == 0 {
		return marshalMCPToolLifecycleResult(mcpToolLifecycleResult{
			Error: fmt.Sprintf("none of the requested tools is a loadable built-in tool in this chat (loadable: %s); pass mcp_server_id to load MCP tools", strings.Join(sortedSetKeys(chatCtx.loadableTools), ", ")),
		})
	}
	current, err := a.ds.ListChatLoadedBuiltinTools(ctx, chatCtx.userID, chatCtx.chat.ID)
	if err != nil {
		return marshalMCPToolLifecycleResult(mcpToolLifecycleResult{Error: err.Error()})
	}
	set := sliceToSet(current)
	var loaded, alreadyLoaded []string
	for _, name := range targets {
		if _, ok := set[name]; ok {
			alreadyLoaded = append(alreadyLoaded, name)
			continue
		}
		set[name] = struct{}{}
		loaded = append(loaded, name)
	}
	if err := a.ds.SetChatLoadedBuiltinTools(ctx, chatCtx.userID, chatCtx.chat.ID, setToSortedSlice(set)); err != nil {
		return marshalMCPToolLifecycleResult(mcpToolLifecycleResult{Error: err.Error()})
	}
	chatCtx.loadedTools = set
	res := mcpToolLifecycleResult{Loaded: loaded, AlreadyLoaded: alreadyLoaded}
	if len(unknown) > 0 {
		res.Note = fmt.Sprintf("Ignored names that are not loadable built-in tools here: %s", strings.Join(unknown, ", "))
	}
	return marshalMCPToolLifecycleResult(res)
}

// resolveRequestedBuiltinTools maps requested names onto the turn's loadable built-in tools
// ("all" takes every one), returning the matches and the unknown names, both sorted.
func resolveRequestedBuiltinTools(chatCtx *chatContext, requested requestedTools) ([]string, []string) {
	if requested.all {
		return sortedSetKeys(chatCtx.loadableTools), nil
	}
	var out, unknown []string
	for _, name := range requested.names {
		if _, ok := chatCtx.loadableTools[name]; ok {
			out = append(out, name)
		} else {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(out)
	sort.Strings(unknown)
	return out, unknown
}

// unloadBuiltinTools removes names from the chat's loaded built-in tools and returns the ones it
// removed. It is not gated on the turn's policy, so stale state can always be cleared.
func (a *Agent) unloadBuiltinTools(ctx context.Context, chatCtx *chatContext, names []string) ([]string, error) {
	current, err := a.ds.ListChatLoadedBuiltinTools(ctx, chatCtx.userID, chatCtx.chat.ID)
	if err != nil {
		return nil, err
	}
	set := sliceToSet(current)
	var removed []string
	for _, name := range names {
		if _, ok := set[name]; ok {
			delete(set, name)
			removed = append(removed, name)
		}
	}
	if len(removed) == 0 {
		return nil, nil
	}
	if err := a.ds.SetChatLoadedBuiltinTools(ctx, chatCtx.userID, chatCtx.chat.ID, setToSortedSlice(set)); err != nil {
		return nil, err
	}
	chatCtx.loadedTools = set
	sort.Strings(removed)
	return removed, nil
}

func sortedSetKeys(set map[string]struct{}) []string {
	out := setToSortedSlice(set)
	if out == nil {
		return []string{}
	}
	return out
}
