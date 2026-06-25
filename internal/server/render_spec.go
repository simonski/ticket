package server

import (
	"encoding/json"
	"strings"
)

// Structured chat rendering (TK-177). A chat/refiner LLM may append a single fenced
// ```render code block containing a JSON render-spec that tells the browser how to
// present its answer (markdown text, a list, a table, or a bar/line chart). The spec
// rides inside the persisted reply text so it survives thread reloads, and a single
// shared front-end renderer (web/shared/app.js) parses the fence out of any agent
// message. The server's only jobs are (a) to instruct the model how to emit the block
// via renderSpecPromptGuidance, and (b) to drop a malformed block before persistence so
// a human never sees raw broken JSON in the transcript (sanitizeRenderBlock).

// renderSpecFenceTag is the info-string that marks a render-spec fenced block.
const renderSpecFenceTag = "render"

// allowedRenderBlockTypes is the whitelist of block types the front-end renderer knows.
var allowedRenderBlockTypes = map[string]bool{
	"text":  true,
	"list":  true,
	"table": true,
	"chart": true,
}

// renderSpecPromptGuidance returns the prompt fragment that teaches the model when and
// how to emit a render block. It is appended to chat/refiner system prompts so the
// capability is described identically wherever it is used.
func renderSpecPromptGuidance() string {
	var b strings.Builder
	b.WriteString("Rich rendering (optional):\n")
	b.WriteString("- When a table, list, or chart would present your answer more clearly than prose, " +
		"you MAY include exactly ONE fenced code block tagged `render` holding a JSON object.\n")
	b.WriteString("- Order your reply: short prose answer FIRST, then the ```render block, then any " +
		"PROPOSE_* marker LAST. Omit the block entirely when plain prose is enough.\n")
	b.WriteString("- Shape: {\"blocks\":[ ... ]} where each block is one of:\n")
	b.WriteString("    {\"type\":\"text\",\"content\":\"<markdown>\"}\n")
	b.WriteString("    {\"type\":\"list\",\"ordered\":false,\"items\":[\"a\",\"b\"]}\n")
	b.WriteString("    {\"type\":\"table\",\"columns\":[\"A\",\"B\"],\"rows\":[[\"1\",\"2\"]]}\n")
	b.WriteString("    {\"type\":\"chart\",\"chartType\":\"bar\",\"labels\":[\"Q1\",\"Q2\"]," +
		"\"series\":[{\"name\":\"x\",\"data\":[1,2]}]}\n")
	b.WriteString("- chartType is \"bar\" or \"line\". Keep all cell/label/series values short. " +
		"Emit valid JSON only inside the block.\n\n")
	return b.String()
}

// renderBlockEnvelope mirrors the minimal shape the front-end renderer expects so the
// server can validate a candidate render block.
type renderBlockEnvelope struct {
	Blocks []struct {
		Type string `json:"type"`
	} `json:"blocks"`
}

// validRenderSpec reports whether jsonText is a well-formed render-spec: a JSON object
// with a non-empty "blocks" array whose every entry carries a known block type.
func validRenderSpec(jsonText string) bool {
	var env renderBlockEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonText)), &env); err != nil {
		return false
	}
	if len(env.Blocks) == 0 {
		return false
	}
	for _, blk := range env.Blocks {
		if !allowedRenderBlockTypes[strings.ToLower(strings.TrimSpace(blk.Type))] {
			return false
		}
	}
	return true
}

// sanitizeRenderBlock returns full unchanged when it carries no render block or a valid
// one, and with the render fence removed when the block is malformed — so a broken
// render-spec degrades to the surrounding prose instead of leaking raw JSON into the
// persisted transcript. Only the first render fence is considered.
func sanitizeRenderBlock(full string) string {
	start, inner, end, ok := findRenderFence(full)
	if !ok || validRenderSpec(inner) {
		return full
	}
	cleaned := full[:start] + full[end:]
	// Collapse the blank gap the removed block may leave behind.
	return strings.TrimRight(cleaned, " \t\n") + "\n"
}

// findRenderFence locates the first ```render fenced block in s. It returns the byte
// offset of the fence opener, the inner JSON text, the offset just past the closing
// fence, and whether a complete block was found.
func findRenderFence(s string) (start int, inner string, end int, ok bool) {
	lines := strings.Split(s, "\n")
	offset := 0
	openLine := -1
	openOffset := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if openLine < 0 {
			if isRenderFenceOpener(trimmed) {
				openLine = i
				openOffset = offset
			}
		} else if trimmed == "```" || trimmed == "~~~" {
			innerStart := openOffset + len(lines[openLine]) + 1
			closeStart := offset
			end := offset + len(line)
			if end < len(s) && s[end] == '\n' {
				end++
			}
			innerText := ""
			if closeStart > innerStart {
				innerText = s[innerStart : closeStart-1]
			}
			return openOffset, innerText, end, true
		}
		offset += len(line) + 1
	}
	return 0, "", 0, false
}

// isRenderFenceOpener reports whether a trimmed line opens a ```render (or ~~~render)
// fenced block.
func isRenderFenceOpener(trimmed string) bool {
	for _, fence := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, fence) {
			info := strings.TrimSpace(trimmed[len(fence):])
			return strings.EqualFold(info, renderSpecFenceTag)
		}
	}
	return false
}
