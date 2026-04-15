// Command tk-test extracts fenced bash code blocks from markdown files and
// executes them sequentially in an isolated environment.  Each block must
// exit 0 to pass.  State (env vars, working directory) carries between
// blocks within the same file, simulating a user following a tutorial.
//
// Usage:
//
//	go run ./cmd/tk-test QUICKSTART_CLIENT.md [QUICKSTART_SERVER.md ...]
//	go run ./cmd/tk-test -ticket ./bin/ticket QUICKSTART_CLIENT.md
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// block represents a single fenced code block extracted from a markdown file.
type block struct {
	file    string
	line    int    // 1-based line number of the opening fence
	code    string // raw content between the fences
	lang    string // language tag (e.g. "bash")
	heading string // most recent markdown heading above this block
}

func main() {
	ticketBin := flag.String("ticket", "", "path to the tk binary (default: ./bin/tk)")
	verbose := flag.Bool("v", false, "print each command before running it")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: tk-test [-ticket ./bin/tk] [-v] <file.md> ...")
		os.Exit(1)
	}

	bin := *ticketBin
	if bin == "" {
		// Default: look for ./bin/tk relative to the working directory.
		bin = "bin/tk"
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(bin); err != nil {
		fmt.Fprintf(os.Stderr, "error: ticket binary not found at %s (run 'make build' first)\n", bin)
		os.Exit(1)
	}

	totalPass, totalFail, totalSkip := 0, 0, 0

	for _, file := range flag.Args() {
		pass, fail, skip, runErr := runFile(file, bin, *verbose)
		totalPass += pass
		totalFail += fail
		totalSkip += skip
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", file, runErr)
		}
	}

	fmt.Println()
	fmt.Printf("Total: %d | Pass: %d | Fail: %d | Skip: %d\n",
		totalPass+totalFail+totalSkip, totalPass, totalFail, totalSkip)
	fmt.Println()

	if totalFail > 0 {
		fmt.Println("RESULT: FAIL")
		os.Exit(1)
	}
	fmt.Println("RESULT: PASS")
}

// runFile processes a single markdown file.  Returns pass/fail/skip counts.
func runFile(file, ticketBin string, verbose bool) (int, int, int, error) {
	blocks, err := parseBlocks(file)
	if err != nil {
		return 0, 0, 0, err
	}

	fmt.Printf("=== %s ===\n\n", file)

	// Create isolated temp environment.
	tmpDir, err := os.MkdirTemp("", "doctest-*")
	if err != nil {
		return 0, 0, 0, err
	}
	defer os.RemoveAll(tmpDir)

	// Initialise a git repo so config.Home() can find .git.
	gitInit := exec.Command("git", "init", tmpDir) // #nosec G204 -- tmpDir is a freshly created temp directory
	gitInit.Stdout = io.Discard
	gitInit.Stderr = io.Discard
	_ = gitInit.Run()

	// Persistent env across blocks within this file.
	env := map[string]string{
		"TICKET_HOME": filepath.Join(tmpDir, ".ticket"),
		"HOME":        tmpDir,
		"PATH":        filepath.Dir(ticketBin) + ":" + os.Getenv("PATH"),
	}

	pass, fail, skip := 0, 0, 0
	var serverCmd *exec.Cmd
	serverPort := 0 // set when a server is started

	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			_ = serverCmd.Process.Kill()
			_ = serverCmd.Wait()
		}
	}()

	for _, b := range blocks {
		if b.lang != "bash" {
			continue
		}

		code := b.code
		label := fmt.Sprintf("%s:%d", b.file, b.line)
		if b.heading != "" {
			label += " (" + b.heading + ")"
		}

		// Skip blocks that are clearly not executable.
		if shouldSkip(code) {
			fmt.Printf("  SKIP  %s\n", label)
			skip++
			continue
		}

		// Rewrite tk/ticket references to use our binary.
		code = rewriteCommands(code, ticketBin)

		// Replace interactive init with initdb for automated quickstart testing.
		if containsInit(code) && !isServerStart(code) {
			code = rewriteInitCommands(code, ticketBin+" initdb")
		}

		// Rewrite hardcoded localhost:8080 to our dynamic port.
		if serverPort > 0 {
			code = strings.ReplaceAll(code, "http://localhost:8080", fmt.Sprintf("http://localhost:%d", serverPort))
		}

		// Detect server start — run in background and wait for healthz.
		if isServerStart(code) {
			if verbose {
				fmt.Printf("  >>    %s\n", strings.TrimSpace(code))
			}
			// If the block contains tk init before tk server, run initdb
			// non-interactively instead (tk init is interactive).
			if containsInit(code) {
				initCode := ticketBin + " initdb"
				if verbose {
					fmt.Printf("  >>    (replacing tk init with: %s)\n", initCode)
				}
				if out, initErr := execBlock(initCode, tmpDir, env); initErr != nil {
					fmt.Printf("  FAIL  %s  |  initdb: %s\n", label, strings.TrimSpace(out))
					fail++
					continue
				}
			}
			// Pick a free port to avoid conflicts.
			port, portErr := freePort()
			if portErr != nil {
				fmt.Printf("  FAIL  %s  |  free port: %v\n", label, portErr)
				fail++
				continue
			}
			serverPort = port
			serverURL := fmt.Sprintf("http://localhost:%d", port)
			env["TICKET_URL"] = serverURL

			serverCmd, err = startServerOnPort(ticketBin, tmpDir, env, port)
			if err != nil {
				fmt.Printf("  FAIL  %s  |  server start: %v\n", label, err)
				fail++
				continue
			}
			if waitHealthz(env, 10*time.Second) {
				fmt.Printf("  PASS  %s  (port %d)\n", label, port)
				pass++
			} else {
				fmt.Printf("  FAIL  %s  |  server not ready after 10s\n", label)
				fail++
			}
			continue
		}

		// Extract export lines to persist in env for subsequent blocks.
		code, newExports := extractExports(code, env)

		// If TICKET_URL is being set, override with our dynamic server URL
		// (the doc may hardcode localhost:8080 but we use a free port).
		if _, ok := newExports["TICKET_URL"]; ok && serverPort > 0 {
			serverURL := fmt.Sprintf("http://localhost:%d", serverPort)
			newExports["TICKET_URL"] = serverURL
			// Also rewrite the export in the code so bash sees the right URL.
			code = strings.ReplaceAll(code, "http://localhost:8080", serverURL)
		}
		// Update config.json location so the CLI detects remote mode.
		if u, ok := newExports["TICKET_URL"]; ok && u != "" {
			updateConfigLocation(env["TICKET_HOME"], u)
		}

		if verbose {
			for _, line := range strings.Split(strings.TrimSpace(code), "\n") {
				fmt.Printf("  >>    %s\n", line)
			}
		}

		// Run the block.
		out, runErr := execBlock(code, tmpDir, env)

		// Apply exports after successful execution.
		if runErr == nil {
			for k, v := range newExports {
				env[k] = v
			}
		}

		if runErr != nil {
			trimmed := strings.TrimSpace(out)
			if len(trimmed) > 200 {
				trimmed = trimmed[:200] + "..."
			}
			fmt.Printf("  FAIL  %s  |  %s\n", label, trimmed)
			fail++
		} else {
			fmt.Printf("  PASS  %s\n", label)
			pass++
		}
	}

	fmt.Println()
	return pass, fail, skip, nil
}

// parseBlocks extracts fenced code blocks from a markdown file.
func parseBlocks(file string) ([]block, error) {
	f, err := os.Open(file) // #nosec G304 -- file is a CLI argument (markdown doc path), not untrusted input
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var blocks []block
	var current *block
	var heading string
	lineNum := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Track headings.
		if strings.HasPrefix(line, "#") {
			heading = strings.TrimSpace(strings.TrimLeft(line, "#"))
			continue
		}

		if current == nil {
			// Look for opening fence.
			if strings.HasPrefix(line, "```") {
				lang := strings.TrimSpace(strings.TrimPrefix(line, "```"))
				current = &block{
					file:    file,
					line:    lineNum,
					lang:    lang,
					heading: heading,
				}
			}
		} else {
			// Inside a fence — look for closing fence.
			if strings.HasPrefix(line, "```") {
				blocks = append(blocks, *current)
				current = nil
			} else {
				if current.code != "" {
					current.code += "\n"
				}
				current.code += line
			}
		}
	}
	return blocks, scanner.Err()
}

// shouldSkip returns true for blocks that should not be executed.
func shouldSkip(code string) bool {
	trimmed := strings.TrimSpace(code)

	// Skip empty blocks.
	if trimmed == "" {
		return true
	}

	// Skip blocks that are purely comments.
	allComments := true
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			allComments = false
			break
		}
	}
	if allComments {
		return true
	}

	// Skip interactive commands.
	if strings.Contains(trimmed, "tk -g") || strings.Contains(trimmed, "tk gui") {
		return true
	}

	// Skip blocks containing placeholder values like <agent-uuid>.
	if strings.Contains(trimmed, "<") && strings.Contains(trimmed, ">") {
		// Check for angle-bracket placeholders (not HTML or redirects).
		for _, line := range strings.Split(trimmed, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Match <word> or <word-word> patterns typical of placeholders.
			for i := 0; i < len(line); i++ {
				if line[i] == '<' {
					end := strings.Index(line[i:], ">")
					if end > 1 {
						inner := line[i+1 : i+end]
						// Looks like a placeholder if it contains letters/hyphens.
						isPlaceholder := true
						for _, c := range inner {
							if c != '-' && c != '_' && c != ' ' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') {
								isPlaceholder = false
								break
							}
						}
						if isPlaceholder && len(inner) > 1 {
							return true
						}
					}
				}
			}
		}
	}

	// Skip blocks that look like output examples (no actual commands).
	// Heuristic: if every non-empty line starts with a known output prefix, skip.
	looksLikeOutput := true
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "No database") &&
			!strings.HasPrefix(line, "admin") &&
			!strings.HasPrefix(line, "  ") &&
			!strings.HasPrefix(line, "#") &&
			!strings.HasPrefix(line, "{") &&
			!strings.HasPrefix(line, "}") &&
			!strings.HasPrefix(line, "\"") &&
			!strings.HasPrefix(line, "|") &&
			!strings.HasPrefix(line, "+") &&
			!strings.HasPrefix(line, "-") {
			looksLikeOutput = false
			break
		}
	}
	if looksLikeOutput && !strings.Contains(trimmed, "export ") {
		// Double-check: if it contains known command prefixes, don't skip.
		for _, line := range strings.Split(trimmed, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "tk ") || strings.HasPrefix(line, "ticket ") ||
				strings.HasPrefix(line, "curl ") || strings.HasPrefix(line, "docker ") ||
				strings.HasPrefix(line, "brew ") || strings.HasPrefix(line, "go ") {
				looksLikeOutput = false
				break
			}
		}
		if looksLikeOutput {
			return true
		}
	}

	// Skip install commands that require external tools.
	if strings.Contains(trimmed, "brew install") ||
		strings.Contains(trimmed, "go install") ||
		strings.Contains(trimmed, "docker") ||
		strings.Contains(trimmed, "ssh ") ||
		strings.Contains(trimmed, "scp ") {
		return true
	}

	return false
}

// containsInit checks whether a code block contains a tk init command.
func containsInit(code string) bool {
	for _, line := range strings.Split(code, "\n") {
		line = strings.TrimSpace(line)
		if line == "tk init" || line == "ticket init" ||
			strings.HasPrefix(line, "tk init ") || strings.HasPrefix(line, "ticket init ") {
			return true
		}
		// Also match the rewritten form.
		if strings.HasSuffix(line, "/ticket init") || strings.Contains(line, "/ticket init ") {
			return true
		}
	}
	return false
}

// isServerStart detects blocks that start the ticket server.
// Matches both original (tk server) and rewritten (/path/to/tk or /path/to/ticket) forms.
func isServerStart(code string) bool {
	for _, line := range strings.Split(code, "\n") {
		line = strings.TrimSpace(line)
		if line == "tk server" || line == "ticket server" ||
			strings.HasPrefix(line, "tk server ") || strings.HasPrefix(line, "ticket server ") {
			return true
		}
		// Match rewritten form: /path/to/tk server or /path/to/ticket server.
		if strings.HasSuffix(line, "/ticket server") || strings.Contains(line, "/ticket server ") ||
			strings.HasSuffix(line, "/tk server") || strings.Contains(line, "/tk server ") {
			return true
		}
	}
	return false
}

// rewriteCommands replaces tk/ticket with the absolute binary path.
func rewriteCommands(code, ticketBin string) string {
	var lines []string
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		// Replace "tk " and "ticket " at start of line (or after export/pipe).
		if strings.HasPrefix(trimmed, "tk ") || trimmed == "tk" {
			line = strings.Replace(line, "tk", ticketBin, 1)
		} else if strings.HasPrefix(trimmed, "ticket ") || trimmed == "ticket" {
			line = strings.Replace(line, "ticket", ticketBin, 1)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func rewriteInitCommands(code, replacement string) string {
	var lines []string
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "tk init", trimmed == "ticket init":
			lines = append(lines, replacement)
		case strings.HasPrefix(trimmed, "tk init "), strings.HasPrefix(trimmed, "ticket init "):
			lines = append(lines, replacement)
		case strings.HasSuffix(trimmed, "/tk init"), strings.HasSuffix(trimmed, "/ticket init"):
			lines = append(lines, replacement)
		case strings.Contains(trimmed, "/tk init "), strings.Contains(trimmed, "/ticket init "):
			lines = append(lines, replacement)
		default:
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// extractExports pulls out `export KEY=VALUE` lines and returns them as a map.
// The exports are also left in the code so the shell sees them during execution.
func extractExports(code string, currentEnv map[string]string) (string, map[string]string) {
	exports := make(map[string]string)
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "export ") {
			rest := strings.TrimPrefix(trimmed, "export ")
			if idx := strings.Index(rest, "="); idx > 0 {
				key := rest[:idx]
				val := rest[idx+1:]
				// Expand $VAR references and strip quotes.
				val = strings.Trim(val, "\"'")
				val = os.Expand(val, func(k string) string {
					if v, ok := currentEnv[k]; ok {
						return v
					}
					return os.Getenv(k)
				})
				exports[key] = val
			}
		}
	}
	return code, exports
}

// execBlock runs a code block as a shell script and returns combined output.
func execBlock(code, workDir string, env map[string]string) (string, error) {
	cmd := exec.Command("bash", "-e", "-c", code) // #nosec G204 -- code is extracted from trusted markdown documentation files
	cmd.Dir = workDir
	cmd.Env = buildEnv(env)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// freePort asks the OS for an available TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port, nil
}

// startServerOnPort runs the ticket server on a specific port in the background.
func startServerOnPort(ticketBin, workDir string, env map[string]string, port int) (*exec.Cmd, error) {
	cmd := exec.Command(ticketBin, "server", "-p", fmt.Sprintf("%d", port)) // #nosec G204 -- ticketBin is a resolved binary path from the build
	cmd.Dir = workDir
	cmd.Env = buildEnv(env)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// waitHealthz polls the server health endpoint until it responds 200 or timeout.
func waitHealthz(env map[string]string, timeout time.Duration) bool {
	// Determine server URL from env or default.
	serverURL := "http://localhost:8080"
	if u, ok := env["TICKET_URL"]; ok && u != "" {
		serverURL = u
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(serverURL + "/api/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// updateConfigLocation writes the location field into config.json so the CLI
// detects remote mode.  This bridges the gap between docs that use
// `export TICKET_URL=...` and the actual config-driven mode resolution.
func updateConfigLocation(ticketHome, location string) {
	configPath := filepath.Join(ticketHome, "config.json")
	data, err := os.ReadFile(configPath) // #nosec G304 -- configPath is derived from a controlled temp directory
	if err != nil {
		// Config doesn't exist yet — create a minimal one.
		data = []byte("{}")
	}
	// Simple JSON manipulation: unmarshal, set, marshal.
	// Avoid importing encoding/json at the top level — it's already imported.
	var m map[string]any
	if jsonErr := json.Unmarshal(data, &m); jsonErr != nil {
		m = make(map[string]any)
	}
	m["location"] = location
	if out, err := json.MarshalIndent(m, "", "  "); err == nil {
		_ = os.MkdirAll(ticketHome, 0o755) // #nosec G301 -- ticketHome is a temp directory; world-readable is intentional for test isolation
		_ = os.WriteFile(configPath, out, 0o600)
	}
}

// buildEnv converts the env map to a slice suitable for exec.Cmd.Env.
func buildEnv(env map[string]string) []string {
	// Start with a minimal set from the OS.
	base := []string{}
	for _, key := range []string{"PATH", "HOME", "USER", "TERM", "TMPDIR", "LANG"} {
		if v := os.Getenv(key); v != "" {
			base = append(base, key+"="+v)
		}
	}

	// Override with our env map.
	seen := make(map[string]bool)
	var result []string
	for k, v := range env {
		result = append(result, k+"="+v)
		seen[k] = true
	}
	for _, entry := range base {
		key := entry[:strings.Index(entry, "=")]
		if !seen[key] {
			result = append(result, entry)
		}
	}
	return result
}
