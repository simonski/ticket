package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/libticket"
)

func statusEnvValue(name string, secret bool) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "UNSET"
	}
	if secret {
		return "********"
	}
	return value
}

// statusLine is a key/value row for the status box.
type statusLine struct {
	key   string
	value string
	color string // ANSI color code prefix, e.g. "\x1b[32m"; empty = default
}

// printStatusBox renders lines inside a rounded Unicode box.
//
// Each line is rendered in two passes: first as a plain string to measure
// visual width, then as a styled string (with any ANSI codes) for printing.
// This keeps the right-hand padding consistent regardless of ANSI content.
func printStatusBox(lines []statusLine) {
	printStatusBoxWidth(lines, 0)
}

func printStatusBoxWidth(lines []statusLine, fixedWidth int) {
	const keyWidth = 17
	const padding = 2 // minimum spaces on each side of content

	// Determine terminal width for capping box width.
	// Non-terminal (piped/tests): no cap. Terminal: use detected width.
	maxContent := 0 // 0 = unlimited
	if isTerminal() {
		termW := 120
		if tw, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && tw > 0 { // #nosec G115
			termW = tw
		}
		maxContent = termW - 2 - padding*2
		if maxContent < 40 {
			maxContent = 40
		}
	}

	type row struct {
		plain  string // visible text, for width measurement
		styled string // text to print (may contain ANSI codes)
	}

	rows := make([]row, len(lines))
	maxWidth := 0
	for i, l := range lines {
		if l.key == "" {
			continue
		}
		plainVal := l.value
		keyPart := fmt.Sprintf("%-*s: ", keyWidth, l.key)
		// Truncate value if the full line would exceed terminal width
		if maxContent > 0 {
			maxVal := maxContent - utf8.RuneCountInString(keyPart)
			if maxVal > 0 && utf8.RuneCountInString(plainVal) > maxVal {
				plainVal = string([]rune(plainVal)[:maxVal-1]) + "…"
			}
		}
		plain := keyPart + plainVal
		styled := plain
		if !noColorOutput && l.color != "" {
			styled = fmt.Sprintf("%-*s: %s%s\x1b[0m", keyWidth, l.key, l.color, plainVal)
		}
		rows[i] = row{plain, styled}
		if w := utf8.RuneCountInString(plain); w > maxWidth {
			maxWidth = w
		}
	}

	// Expand to fill the terminal width (or fixedWidth if provided).
	targetWidth := 0
	if fixedWidth > 0 {
		targetWidth = fixedWidth
	} else if isTerminal() {
		if tw, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && tw > 0 { // #nosec G115
			targetWidth = tw
		}
	}
	if targetWidth > 0 {
		contentW := targetWidth - 2 - padding*2 // subtract borders and padding
		if contentW > maxWidth {
			maxWidth = contentW
		}
	}
	inner := maxWidth + padding*2

	fmt.Println("╭" + strings.Repeat("─", inner) + "╮")
	for i, l := range lines {
		if l.key == "" {
			fmt.Println("│" + strings.Repeat(" ", inner) + "│")
			continue
		}
		r := rows[i]
		rightPad := inner - padding - utf8.RuneCountInString(r.plain)
		if rightPad < 0 {
			rightPad = 0
		}
		fmt.Printf("│%s%s%s│\n",
			strings.Repeat(" ", padding),
			r.styled,
			strings.Repeat(" ", rightPad))
	}
	fmt.Println("╰" + strings.Repeat("─", inner) + "╯")
}

func runRemoteStatusWithSummaryStyle(cfg config.Config, _ bool) error {
	serverURL, _, err := currentConfiguredRemoteServer()
	if err != nil {
		return err
	}
	statusCfg, username, passwordDisplay, passwordColor := remoteStatusConfig(cfg, serverURL)
	statusErr := error(nil)
	connected := false
	if strings.TrimSpace(serverURL) != "" {
		svc := libticket.NewHTTP(statusCfg)
		status, err := svc.Status(context.Background())
		statusErr = err
		connected = err == nil
		if connected && status.User != nil && strings.TrimSpace(status.User.Username) != "" {
			username = strings.TrimSpace(status.User.Username)
		}
	}
	urlColor := "\x1b[31m"
	if connected {
		urlColor = "\x1b[32m"
	}
	usernameColor := "\x1b[31m"
	if strings.TrimSpace(username) != "" && strings.TrimSpace(username) != "UNSET" {
		usernameColor = "\x1b[32m"
	}
	if outputJSON {
		payload := map[string]any{
			"TICKET_URL":      serverURL,
			"TICKET_USERNAME": valueOrDefault(username, "UNSET"),
			"TICKET_PASSWORD": passwordDisplay,
		}
		return printJSON(payload)
	}
	lines := []statusLine{
		{key: "TICKET_URL", value: valueOrDefault(serverURL, "UNSET"), color: urlColor},
		{key: "TICKET_USERNAME", value: valueOrDefault(username, "UNSET"), color: usernameColor},
		{key: "TICKET_PASSWORD", value: passwordDisplay, color: passwordColor},
	}
	printStatusBox(lines)
	return statusErr
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func remoteStatusConfig(cfg config.Config, serverURL string) (statusCfg config.Config, username, passwordDisplay, passwordColor string) {
	statusCfg = config.Config{Location: strings.TrimSpace(serverURL)}
	username = strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
	password := strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
	token := strings.TrimSpace(os.Getenv("TICKET_TOKEN"))
	switch {
	case token != "":
		statusCfg.Token = token
		return statusCfg, valueOrDefault(username, strings.TrimSpace(cfg.Username)), "********", "\x1b[32m"
	case username != "" && password != "":
		statusCfg.Username = username
		statusCfg.Token = password
		statusCfg.UseBasicAuth = true
		return statusCfg, username, "********", "\x1b[32m"
	case strings.TrimSpace(cfg.Token) != "":
		statusCfg.Username = strings.TrimSpace(cfg.Username)
		statusCfg.Token = strings.TrimSpace(cfg.Token)
		return statusCfg, strings.TrimSpace(cfg.Username), "********", "\x1b[32m"
	default:
		return statusCfg, valueOrDefault(username, strings.TrimSpace(cfg.Username)), "UNSET", "\x1b[31m"
	}
}
