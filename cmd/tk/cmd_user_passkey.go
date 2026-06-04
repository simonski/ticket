package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

var (
	passkeyBrowserOpener = openBrowserURL
	passkeyPollInterval  = 2 * time.Second
)

func runPasskeyLogin(cfg config.Config, serverURL, usernameFlag string) error {
	username, err := resolvePasskeyUsername(cfg.Username, usernameFlag)
	if err != nil {
		return err
	}
	api := client.New(config.Config{Location: serverURL})
	start, err := api.StartPasskeyLogin(context.Background(), client.PasskeyLoginStartRequest{Username: username})
	if err != nil {
		return err
	}
	announcePasskeyFlow(start, "login")
	response, err := waitForPasskey(api, start.Code, start.ExpiresAt)
	if err != nil {
		return err
	}
	if response.User == nil || strings.TrimSpace(response.Token) == "" {
		return errors.New("passkey login completed without a session token")
	}
	return finishLogin(cfg, *response.User, response.Token)
}

func runUserPasskey(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(userPasskeyUsage)
		return nil
	}
	switch args[0] {
	case "enroll", "add":
		fs := flag.NewFlagSet("user passkey enroll", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		nameFlag := fs.String("name", "", "passkey label")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if strings.TrimSpace(cfg.Token) == "" {
			return errors.New("passkey enrollment requires an existing login session; run tk login first")
		}
		serverURL, err := resolveServerURLForAuth(cfg)
		if err != nil {
			return err
		}
		api := client.New(config.Config{Location: serverURL, Token: cfg.Token})
		start, err := api.StartPasskeyRegistration(context.Background(), client.PasskeyRegistrationStartRequest{
			Name: strings.TrimSpace(*nameFlag),
		})
		if err != nil {
			return err
		}
		announcePasskeyFlow(start, "enrollment")
		response, err := waitForPasskey(api, start.Code, start.ExpiresAt)
		if err != nil {
			return err
		}
		if response.Status != store.PasskeyFlowStatusComplete {
			return errors.New("passkey enrollment did not complete")
		}
		if outputJSON {
			return printJSON(map[string]string{"status": response.Status})
		}
		fmt.Fprintln(os.Stdout, "passkey enrolled")
		return nil
	default:
		return fmt.Errorf("unknown user passkey command %q; see: ticket user passkey help", args[0])
	}
}

func resolvePasskeyUsername(currentUsername, explicitUsername string) (string, error) {
	username := resolveLoginUsername(currentUsername, explicitUsername)
	if strings.TrimSpace(username) != "" {
		return strings.TrimSpace(username), nil
	}
	return promptForPasskeyUsername(loginPromptInput, loginPromptOutput)
}

func promptForPasskeyUsername(input io.Reader, output io.Writer) (string, error) {
	reader := bufio.NewReader(input)
	fmt.Fprint(output, "Username: ")
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	username := strings.TrimSpace(value)
	if username == "" {
		return "", errors.New("username is required")
	}
	return username, nil
}

func announcePasskeyFlow(start client.PasskeyStartResponse, action string) {
	writer := os.Stdout
	if outputJSON {
		writer = os.Stderr
	}
	fmt.Fprintf(writer, "passkey %s URL: %s\n", action, start.VerificationURL)
	fmt.Fprintf(writer, "passkey code: %s\n", start.Code)
	if err := passkeyBrowserOpener(start.VerificationURL); err == nil {
		fmt.Fprintln(writer, "opened browser for passkey verification")
	}
}

func waitForPasskey(api *client.Client, code, expiresAt string) (client.PasskeyPollResponse, error) {
	deadline := time.Now().Add(10 * time.Minute)
	if parsed, err := parsePasskeyExpiry(expiresAt); err == nil {
		deadline = parsed
	}
	for {
		response, err := api.PollPasskey(context.Background(), code)
		if err == nil {
			if strings.TrimSpace(response.Status) == store.PasskeyFlowStatusComplete {
				return response, nil
			}
		} else {
			var statusErr *client.HTTPStatusError
			if !(errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusAccepted) {
				return client.PasskeyPollResponse{}, err
			}
		}
		if time.Now().After(deadline) {
			return client.PasskeyPollResponse{}, errors.New("passkey verification expired before completion")
		}
		time.Sleep(passkeyPollInterval)
	}
}

func parsePasskeyExpiry(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, errors.New("expiry is required")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("unsupported expiry format")
}

func openBrowserURL(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("browser url is required")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target) // #nosec G204 -- command and args are fixed
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target) // #nosec G204 -- command and args are fixed
	default:
		cmd = exec.Command("xdg-open", target) // #nosec G204 -- command and args are fixed
	}
	return cmd.Start()
}
