package main

import (
	"errors"
	"fmt"
	"strings"
)

const dockerComposeTemplate = `services:
  ticket:
    image: ghcr.io/simonski/ticket:latest
    labels:
      - com.centurylinklabs.watchtower.enable=true
    environment:
      TICKET_DATA_DIR: /data
      TICKET_DB_PATH: /data/ticket.db
      TICKET_SERVER_ADDR: 0.0.0.0:8080
      TICKET_ADMIN_PASSWORD: password
    ports:
      - "8080:8080"
    volumes:
      - ticket-data:/data
    restart: unless-stopped
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/api/healthz"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3
  watchtower:
    image: containrrr/watchtower:latest
    command:
      - --label-enable
      - --cleanup
      - --interval
      - "30"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    restart: unless-stopped

volumes:
  ticket-data:
`

func runDockerCompose(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tk docker-compose")
	}
	if outputJSON {
		return printJSON(map[string]string{
			"status":  "ok",
			"type":    "docker-compose",
			"content": strings.TrimSpace(dockerComposeTemplate) + "\n",
		})
	}
	fmt.Print(strings.TrimSpace(dockerComposeTemplate))
	if !strings.HasSuffix(dockerComposeTemplate, "\n") {
		fmt.Println()
	}
	return nil
}
