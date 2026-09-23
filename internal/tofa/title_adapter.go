package tofa

import (
	"encoding/json"
	"errors"
	"reflect"
)

// Captured from desktop 26.915.31945 / engine 0.155.0-alpha.9.2.
// The desktop independently parses JSON and validates title and description.
const desktopTitleSchema = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"title":{"type":"string","minLength":1,"maxLength":36},"description":{"type":"string","minLength":1}},"required":["title","description"],"additionalProperties":false}`

func adaptDesktopTitle(body []byte) ([]byte, bool, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, err
	}
	var model string
	json.Unmarshal(payload["model"], &model)
	if model != "moonshotai/Kimi-K3" {
		return body, false, nil
	}
	var metadata map[string]json.RawMessage
	var turnJSON string
	var turn struct {
		Source  string `json:"thread_source"`
		Trigger string `json:"turn_trigger"`
	}
	json.Unmarshal(payload["client_metadata"], &metadata)
	json.Unmarshal(metadata["x-codex-turn-metadata"], &turnJSON)
	if json.Unmarshal([]byte(turnJSON), &turn) != nil || turn.Source != "thread_title" || turn.Trigger != "thread_title" {
		return body, false, nil
	}
	unsupported := errors.New("unsupported Kimi-K3 desktop title contract; request was not sent upstream")
	var textOptions, format map[string]json.RawMessage
	if json.Unmarshal(payload["text"], &textOptions) != nil || json.Unmarshal(textOptions["format"], &format) != nil {
		return nil, false, unsupported
	}
	var schema, expected any
	json.Unmarshal(format["schema"], &schema)
	json.Unmarshal([]byte(desktopTitleSchema), &expected)
	if !reflect.DeepEqual(schema, expected) {
		return nil, false, unsupported
	}
	var kind, name, choice string
	var strict *bool
	json.Unmarshal(format["type"], &kind)
	json.Unmarshal(format["name"], &name)
	json.Unmarshal(payload["tool_choice"], &choice)
	if len(format) != 4 || kind != "json_schema" || name != "codex_output_schema" || json.Unmarshal(format["strict"], &strict) != nil || strict == nil || !*strict || choice != "auto" {
		return nil, false, unsupported
	}
	var tools []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Tools json.RawMessage `json:"tools"`
	}
	allowed := map[string]bool{
		"exec_command": true, "write_stdin": true, "list_mcp_resources": true,
		"list_mcp_resource_templates": true, "read_mcp_resource": true,
		"request_user_input": true, "view_image": true, "mcp__node_repl": true,
		"get_goal": true, "create_goal": true, "update_goal": true,
	}
	if json.Unmarshal(payload["tools"], &tools) != nil || len(tools) != len(allowed) {
		return nil, false, unsupported
	}
	for _, tool := range tools {
		if !allowed[tool.Name] {
			return nil, false, unsupported
		}
		delete(allowed, tool.Name)
		if tool.Name == "mcp__node_repl" {
			var members []struct {
				Type string `json:"type"`
				Name string `json:"name"`
			}
			names := map[string]bool{"js": true, "js_add_node_module_dir": true, "js_reset": true}
			if tool.Type != "namespace" || json.Unmarshal(tool.Tools, &members) != nil || len(members) != len(names) {
				return nil, false, unsupported
			}
			for _, member := range members {
				if member.Type != "function" || !names[member.Name] {
					return nil, false, unsupported
				}
				delete(names, member.Name)
			}
		} else if tool.Type != "function" || len(tool.Tools) != 0 {
			return nil, false, unsupported
		}
	}
	var instructions *string
	if json.Unmarshal(payload["instructions"], &instructions) != nil || instructions == nil {
		return nil, false, unsupported
	}
	// Preserve tools: title generation can require read-only app lookups. Only
	// the constrained-decoding location changes; the full schema remains guidance.
	canonicalSchema, _ := json.Marshal(schema)
	*instructions += "\n\nFor this desktop thread title, return your final answer as JSON matching this complete schema (no markdown or extra text):\n" + string(canonicalSchema)
	payload["instructions"], _ = json.Marshal(instructions)
	delete(textOptions, "format")
	payload["text"], _ = json.Marshal(textOptions)
	result, err := json.Marshal(payload)
	return result, true, err
}
