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

func promptForCredentials(in io.Reader, out io.Writer, defaultUsername, defaultPassword string) (string, string, error) {
	reader := bufio.NewReader(in)
	if defaultUsername != "" {
		fmt.Fprintf(out, "username [%s]: ", defaultUsername)
	} else {
		fmt.Fprint(out, "username: ")
	}
	username, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = defaultUsername
	}
	if defaultPassword != "" {
		fmt.Fprint(out, "password [press enter to use default]: ")
	} else {
		fmt.Fprint(out, "password: ")
	}
	password, err := readPasswordPrompt(reader, in, out)
	if err != nil {
		return "", "", err
	}
	if password == "" {
		password = defaultPassword
	}
	return username, password, nil
}

func readPasswordPrompt(reader *bufio.Reader, in io.Reader, out io.Writer) (string, error) {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK || !term.IsTerminal(int(inFile.Fd())) || !term.IsTerminal(int(outFile.Fd())) {
		password, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(password), nil
	}

	oldState, err := term.MakeRaw(int(inFile.Fd()))
	if err != nil {
		return "", err
	}
	defer func() {
		_ = term.Restore(int(inFile.Fd()), oldState)
	}()

	var buf []byte
	single := make([]byte, 1)
	for {
		if _, err := inFile.Read(single); err != nil {
			return "", err
		}
		switch single[0] {
		case '\r', '\n':
			fmt.Fprint(out, "\n")
			return string(buf), nil
		case 3:
			fmt.Fprint(out, "^C\n")
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

func resolveLoginUsername(configUsername, usernameFlag string) string {
	if strings.TrimSpace(configUsername) != "" {
		return strings.TrimSpace(configUsername)
	}
	if strings.TrimSpace(usernameFlag) != "" {
		return strings.TrimSpace(usernameFlag)
	}
	return envValue("TICKET_USERNAME")
}

func resolveLoginPassword(passwordFlag string) string {
	if strings.TrimSpace(passwordFlag) != "" {
		return strings.TrimSpace(passwordFlag)
	}
	return envValue("TICKET_PASSWORD")
}
