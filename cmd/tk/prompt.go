package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// errPromptInterrupted is returned when the user presses Ctrl-C at a prompt.
var errPromptInterrupted = errors.New("cancelled")

func promptForCredentials(in io.Reader, out io.Writer, defaultUsername, defaultPassword string) (username, password string, err error) {
	usernameLabel := "username: "
	if defaultUsername != "" {
		usernameLabel = fmt.Sprintf("username [%s]: ", defaultUsername)
	}
	passwordLabel := "password: "
	if defaultPassword != "" {
		passwordLabel = "password [press enter to use default]: "
	}

	// On a real terminal, read both fields in raw mode with a single MakeRaw.
	// This filters terminal escape sequences (e.g. OSC 11 / cursor-position query
	// responses emitted by color detection at startup, which would otherwise leak
	// into the read) and handles Ctrl-C consistently for both prompts (TK-162).
	if inFile, _, ok := promptTerminal(in, out); ok {
		oldState, rawErr := term.MakeRaw(int(inFile.Fd())) // #nosec G115
		if rawErr != nil {
			return "", "", rawErr
		}
		defer func() {
			if restoreErr := term.Restore(int(inFile.Fd()), oldState); restoreErr != nil { // #nosec G115
				fmt.Fprintf(out, "warning: failed to restore terminal state: %v\r\n", restoreErr)
			}
		}()
		fmt.Fprint(out, usernameLabel)
		username, err = readRawLine(inFile, out, false)
		if err != nil {
			return "", "", err
		}
		username = strings.TrimSpace(username)
		if username == "" {
			username = defaultUsername
		}
		fmt.Fprint(out, passwordLabel)
		password, err = readRawLine(inFile, out, true)
		if err != nil {
			return "", "", err
		}
		if password == "" {
			password = defaultPassword
		}
		return username, password, nil
	}

	// Non-terminal fallback (pipes/tests): line-based reads.
	reader := bufio.NewReader(in)
	fmt.Fprint(out, usernameLabel)
	username, err = reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = defaultUsername
	}
	fmt.Fprint(out, passwordLabel)
	password, err = readPasswordPrompt(reader, in, out)
	if err != nil {
		return "", "", err
	}
	if password == "" {
		password = defaultPassword
	}
	return username, password, nil
}

// promptTerminal reports whether in/out are a usable interactive terminal.
func promptTerminal(in io.Reader, out io.Writer) (inFile, outFile *os.File, ok bool) {
	inF, inOK := in.(*os.File)
	outF, outOK := out.(*os.File)
	if !inOK || !outOK {
		return nil, nil, false
	}
	if !term.IsTerminal(int(inF.Fd())) || !term.IsTerminal(int(outF.Fd())) { // #nosec G115
		return nil, nil, false
	}
	return inF, outF, true
}

// readRawLine reads one line from a terminal already in raw mode. It echoes input
// (asterisks when mask is set), supports backspace, returns errPromptInterrupted
// on Ctrl-C, and silently discards terminal escape sequences (leaked query
// responses) so they don't corrupt the value (TK-162).
func readRawLine(inFile *os.File, out io.Writer, mask bool) (string, error) {
	var buf []byte
	single := make([]byte, 1)
	for {
		if _, err := inFile.Read(single); err != nil {
			return "", err
		}
		switch single[0] {
		case '\r', '\n':
			fmt.Fprint(out, "\r\n")
			return string(buf), nil
		case 3: // Ctrl-C
			fmt.Fprint(out, "^C\r\n")
			return "", errPromptInterrupted
		case 8, 127: // backspace / delete
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Fprint(out, "\b \b")
			}
		case 0x1b: // ESC — consume and ignore a terminal escape/query response
			consumeEscapeSequence(inFile)
		default:
			if single[0] >= 32 && single[0] <= 126 {
				buf = append(buf, single[0])
				if mask {
					fmt.Fprint(out, "*")
				} else {
					fmt.Fprint(out, string(single[0]))
				}
			}
		}
	}
}

// consumeEscapeSequence reads and discards the remainder of a terminal escape
// sequence after an ESC byte: CSI (\x1b[ … final byte 0x40-0x7e), OSC (\x1b] …
// terminated by BEL or ST), or a single-byte escape.
func consumeEscapeSequence(inFile *os.File) {
	b := make([]byte, 1)
	if _, err := inFile.Read(b); err != nil {
		return
	}
	switch b[0] {
	case '[': // CSI
		for {
			if _, err := inFile.Read(b); err != nil {
				return
			}
			if b[0] >= 0x40 && b[0] <= 0x7e {
				return
			}
		}
	case ']': // OSC, terminated by BEL (0x07) or ST (ESC \)
		for {
			if _, err := inFile.Read(b); err != nil {
				return
			}
			if b[0] == 0x07 {
				return
			}
			if b[0] == 0x1b {
				_, _ = inFile.Read(b) // consume the trailing backslash of ST
				return
			}
		}
	default:
		return
	}
}

func readPasswordPrompt(reader *bufio.Reader, in io.Reader, out io.Writer) (string, error) {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK || !term.IsTerminal(int(inFile.Fd())) || !term.IsTerminal(int(outFile.Fd())) { // #nosec G115 -- uintptr→int is safe for terminal fd on all supported platforms
		password, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(password), nil
	}

	oldState, err := term.MakeRaw(int(inFile.Fd())) // #nosec G115
	if err != nil {
		return "", err
	}
	defer func() {
		if restoreErr := term.Restore(int(inFile.Fd()), oldState); restoreErr != nil { // #nosec G115
			fmt.Fprintf(out, "warning: failed to restore terminal state: %v\n", restoreErr)
		}
	}()

	var buf []byte
	single := make([]byte, 1)
	for {
		if _, err := inFile.Read(single); err != nil {
			return "", err
		}
		switch single[0] {
		case '\r', '\n':
			// Raw mode: \n is line-feed only, so emit \r\n to return the cursor to
			// column 0 — otherwise the next output is indented by the password
			// length (the count of asterisks just printed) (TK-161).
			fmt.Fprint(out, "\r\n")
			return string(buf), nil
		case 3:
			fmt.Fprint(out, "^C\r\n")
			return "", errors.New("interrupt")
		case 8, 127:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Fprint(out, "\b \b")
			}
		default:
			if single[0] >= 32 && single[0] <= 126 {
				buf = append(buf, single[0])
				fmt.Fprint(out, "*")
			}
		}
	}
}

func promptChoiceWithDefault(reader *bufio.Reader, question string, options []string, defaultIdx int) int {
	fmt.Println(question)
	for i, opt := range options {
		fmt.Printf("  %d. %s\n", i+1, opt)
	}
	for {
		fmt.Printf("choice [%d]: ", defaultIdx+1)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return defaultIdx
		}
		var n int
		if _, err := fmt.Sscanf(line, "%d", &n); err == nil && n >= 1 && n <= len(options) {
			return n - 1
		}
		fmt.Printf("please enter a number between 1 and %d\n", len(options))
	}
}

func resolveLoginUsername(configUsername, usernameFlag string) string {
	if strings.TrimSpace(configUsername) != "" {
		return strings.TrimSpace(configUsername)
	}
	if strings.TrimSpace(usernameFlag) != "" {
		return strings.TrimSpace(usernameFlag)
	}
	return ""
}

func resolveLoginPassword(passwordFlag string) string {
	if strings.TrimSpace(passwordFlag) != "" {
		return strings.TrimSpace(passwordFlag)
	}
	return ""
}
