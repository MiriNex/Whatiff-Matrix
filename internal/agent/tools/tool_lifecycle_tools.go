package tools

const LoadToolsDescription = `Load tools into this chat's active toolset. Loaded tools stay available across future turns in this chat until you unload them.

Two kinds of tools are loaded this way:
- Built-in tools: omit mcp_server_id and pass the names from the loadable built-in tools listed in your instructions (for example web_search or generate_image). Pass "all" or "*" to load all of them.
- MCP tools: set mcp_server_id to the connector UUID from list(kind="mcp_servers") and pass that connector's tool names. You can pass canonical full MCP names (mcp__<connector>__<tool>) or connector-local names. Pass "all" or "*" to load every discoverable tool from that connector.

A loaded tool can be called from your next step in this same turn.`

const UnloadToolsDescription = `Unload tools from this chat's active toolset.

Loaded tools persist across turns; call this to remove tools you no longer need.

Inputs:
- mcp_server_id (optional): connector UUID to target.
- tools (required): array of names to unload, or include "all"/"*" to clear everything.

Behavior:
- tool names + no mcp_server_id: unload those built-in or MCP tools wherever they are loaded.
- tools includes "all" or "*" + mcp_server_id set: unload all tools for that connector.
- tools includes "all" or "*" + no mcp_server_id: unload every loaded tool in this chat, built-in and MCP.`

var LoadToolsToolSpec = FunctionToolSpec{
	Name:        "load_tools",
	Description: LoadToolsDescription,
	Properties: map[string]interface{}{
		"mcp_server_id": map[string]interface{}{
			"type":        "string",
			"description": "UUID of the MCP connector to load tools from (from list kind=\"mcp_servers\"). Omit to load built-in tools.",
		},
		"tools": map[string]interface{}{
			"type":        "array",
			"description": "Tool names to load for this chat, or [\"all\"] / [\"*\"] for every loadable tool of that kind.",
			"items": map[string]interface{}{
				"type": "string",
			},
			"minItems": 1,
		},
	},
	Required: []string{"tools"},
}

var UnloadToolsToolSpec = FunctionToolSpec{
	Name:        "unload_tools",
	Description: UnloadToolsDescription,
	Properties: map[string]interface{}{
		"mcp_server_id": map[string]interface{}{
			"type":        "string",
			"description": "Optional UUID of one MCP connector to target.",
		},
		"tools": map[string]interface{}{
			"type":        "array",
			"description": "Tool names to unload, or [\"all\"] / [\"*\"] to unload everything (connector-scoped when mcp_server_id is set; every loaded tool otherwise).",
			"items": map[string]interface{}{
				"type": "string",
			},
			"minItems": 1,
		},
	},
	Required: []string{"tools"},
}
