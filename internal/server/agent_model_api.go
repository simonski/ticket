package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/store"
)

// agentModelCanCallAPI reports whether the resolved config points at an HTTP API
// (vs a CLI provider, which has no base URL). API providers carry a base_url and
// an API key.
func agentModelCanCallAPI(cfg store.AgentModelConfig) bool {
	return strings.TrimSpace(cfg.URL) != "" && strings.TrimSpace(cfg.APIKey) != "" && strings.TrimSpace(cfg.Model) != ""
}

// callAgentModelAPI sends a single-prompt completion request to the configured
// provider (Anthropic Messages API, or an OpenAI-compatible /chat/completions
// endpoint) and returns the assistant's text (TK-149).
func callAgentModelAPI(ctx context.Context, cfg store.AgentModelConfig, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if strings.EqualFold(cfg.Provider, "anthropic") || strings.Contains(strings.ToLower(cfg.URL), "anthropic") {
		return callAnthropic(ctx, cfg, prompt)
	}
	return callOpenAICompatible(ctx, cfg, prompt)
}

func httpJSON(ctx context.Context, method, url string, headers map[string]string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("%s %s -> %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func callAnthropic(ctx context.Context, cfg store.AgentModelConfig, prompt string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	url := base
	if !strings.HasSuffix(url, "/v1/messages") {
		url += "/v1/messages"
	}
	body := map[string]any{
		"model":      cfg.Model,
		"max_tokens": 1024,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	out, err := httpJSON(ctx, http.MethodPost, url, map[string]string{
		"x-api-key":         cfg.APIKey,
		"anthropic-version": "2023-06-01",
	}, body)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if jerr := json.Unmarshal(out, &parsed); jerr != nil {
		return "", jerr
	}
	var b strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func callOpenAICompatible(ctx context.Context, cfg store.AgentModelConfig, prompt string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	url := base
	if !strings.HasSuffix(url, "/chat/completions") {
		url += "/chat/completions"
	}
	body := map[string]any{
		"model":    cfg.Model,
		"messages": []map[string]any{{"role": "user", "content": prompt}},
	}
	headers := map[string]string{}
	if strings.TrimSpace(cfg.APIKey) != "" {
		headers["Authorization"] = "Bearer " + cfg.APIKey
	}
	out, err := httpJSON(ctx, http.MethodPost, url, headers, body)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if jerr := json.Unmarshal(out, &parsed); jerr != nil {
		return "", jerr
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("model returned no choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// claudeJSONEvent is one line of Claude CLI --output-format (stream-)json output.
type claudeJSONEvent struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// extractAgentReply turns a CLI command's stdout into the assistant text. For a
// command using Claude's --output-format stream-json it parses the NDJSON events
// line by line (preferring the final result, else concatenated assistant text);
// for --output-format json it parses the single result object; otherwise it
// returns the raw stdout (TK-158).
func extractAgentReply(cmdArgs []string, raw string) string {
	joined := strings.ToLower(strings.Join(cmdArgs, " "))
	if strings.Contains(joined, "stream-json") {
		if text := parseClaudeStreamJSON(raw); text != "" {
			return text
		}
		return strings.TrimSpace(raw)
	}
	if strings.Contains(joined, "output-format json") {
		var ev claudeJSONEvent
		if json.Unmarshal([]byte(strings.TrimSpace(raw)), &ev) == nil {
			if t := claudeEventText(ev); t != "" {
				return t
			}
		}
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(raw)
}

func parseClaudeStreamJSON(raw string) string {
	var result string
	var acc strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var ev claudeJSONEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		if ev.Type == "result" && strings.TrimSpace(ev.Result) != "" {
			result = ev.Result
		} else if ev.Type == "assistant" {
			for _, c := range ev.Message.Content {
				if c.Type == "text" {
					acc.WriteString(c.Text)
				}
			}
		}
	}
	if strings.TrimSpace(result) != "" {
		return strings.TrimSpace(result)
	}
	return strings.TrimSpace(acc.String())
}

func claudeEventText(ev claudeJSONEvent) string {
	if strings.TrimSpace(ev.Result) != "" {
		return strings.TrimSpace(ev.Result)
	}
	var acc strings.Builder
	for _, c := range ev.Message.Content {
		if c.Type == "text" {
			acc.WriteString(c.Text)
		}
	}
	return strings.TrimSpace(acc.String())
}
