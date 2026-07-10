package service

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestArcaHTTP_logsRequestAndResponse_withRedactedToken(t *testing.T) {
	var logs bytes.Buffer
	logger := log.StandardLogger()
	oldOutput := logger.Out
	oldFormatter := logger.Formatter
	oldLevel := logger.Level
	logger.SetOutput(&logs)
	logger.SetFormatter(&log.TextFormatter{DisableTimestamp: true, DisableColors: true})
	logger.SetLevel(log.InfoLevel)
	t.Cleanup(func() {
		logger.SetOutput(oldOutput)
		logger.SetFormatter(oldFormatter)
		logger.SetLevel(oldLevel)
	})

	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"token":"abc123"}`)
	})
	c := m.client()

	_, err := c.PresignURL("sb-1", 8081, 60)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := logs.String()
	if !strings.Contains(output, "Arca API request") {
		t.Fatalf("logs missing request: %s", output)
	}
	if !strings.Contains(output, "Arca API response") {
		t.Fatalf("logs missing response: %s", output)
	}
	if !strings.Contains(output, "/arca/api/v1/sandbox/sb-1/presign/token") {
		t.Fatalf("logs missing presign path: %s", output)
	}
	if strings.Contains(output, "abc123") {
		t.Fatalf("logs leaked presign token: %s", output)
	}
	if !strings.Contains(output, "<redacted>") {
		t.Fatalf("logs missing redaction marker: %s", output)
	}
}
