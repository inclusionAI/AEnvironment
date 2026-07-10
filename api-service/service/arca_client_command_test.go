package service

import (
	"net/http"
	"testing"
)

func TestArcaCreate_forwardsCommand_whenInitCommandIsSet(t *testing.T) {
	m := newArcaMock(t, func(r *http.Request) (int, string) {
		return http.StatusOK, okResponse(`{"sandbox_id":"sb-command"}`)
	})
	c := m.client()
	env := sampleEnv(map[string]interface{}{
		"arcaTemplateId": "tpl1",
		"initCommand":    "python3 -m http.server 8081",
	})

	_, err := c.CreateEnvInstance(env)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := decodeBody(t, m.lastRequest(t).Body)
	if body["command"] != "python3 -m http.server 8081" {
		t.Fatalf("command = %v, want init command", body["command"])
	}
}
