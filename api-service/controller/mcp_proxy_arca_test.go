package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-service/constants"

	"github.com/gin-gonic/gin"
)

func TestMCPGateway_ArcaProxy_forwardsRequestViaOneHourPresignedURL(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)

	var presignCalled bool
	var sessionCalled bool
	arca := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/arca/api/v1/sandbox/sb-1/presign/token":
			presignCalled = true
			if r.Method != http.MethodPost {
				t.Errorf("presign method = %q, want POST", r.Method)
			}
			if r.Header.Get("x-agent-sandbox-id") != "sb-1" {
				t.Errorf("presign sandbox header = %q, want sb-1", r.Header.Get("x-agent-sandbox-id"))
			}
			if r.Header.Get("x-agent-sandbox-port") != "18080" {
				t.Errorf("presign port header = %q, want 18080", r.Header.Get("x-agent-sandbox-port"))
			}
			var body map[string]float64
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode presign body: %v", err)
			}
			if body["expiration_time"] != 60 {
				t.Errorf("expiration_time = %v, want 60", body["expiration_time"])
			}
			_, _ = w.Write([]byte(`{"success":true,"code":"","message":"","data":{"token":"tok-1"}}`))
		case "/arca/api/v1/session/tok-1/task/reward":
			sessionCalled = true
			if r.Method != http.MethodPost {
				t.Errorf("session method = %q, want POST", r.Method)
			}
			if r.URL.RawQuery != "case=1" {
				t.Errorf("session query = %q, want case=1", r.URL.RawQuery)
			}
			if r.Header.Get(constants.HeaderEnvInstanceID) != "" {
				t.Errorf("internal instance header was forwarded")
			}
			if r.Header.Get(constants.HeaderMCPServerURL) != "" {
				t.Errorf("internal proxy URL header was forwarded")
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read session body: %v", err)
			}
			if string(body) != `{"ok":true}` {
				t.Errorf("session body = %q, want JSON payload", string(body))
			}
			w.Header().Set("X-Arca-Upstream", "ok")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			t.Errorf("unexpected arca path: %s", r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer arca.Close()

	router := gin.New()
	NewMCPGateway(router.Group("/"), MCPGatewayConfig{
		ScheduleType: scheduleTypeArca,
		ScheduleAddr: arca.URL,
		ArcaAPIKey:   "test-key",
	})

	req := httptest.NewRequest(http.MethodPost, "/task/reward?case=1", strings.NewReader(`{"ok":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(constants.HeaderEnvInstanceID, "sb-1")
	req.Header.Set(constants.HeaderMCPServerURL, "http://127.0.0.1:18080")
	w := newCloseNotifyRecorder()

	// When
	router.ServeHTTP(w, req)

	// Then
	if !presignCalled {
		t.Fatalf("presign endpoint was not called")
	}
	if !sessionCalled {
		t.Fatalf("session endpoint was not called")
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	if w.Header().Get("X-Arca-Upstream") != "ok" {
		t.Errorf("missing upstream response header")
	}
}

func TestMCPGateway_ArcaProxy_returnsBadRequest_whenInstanceIDMissing(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewMCPGateway(router.Group("/"), MCPGatewayConfig{
		ScheduleType: scheduleTypeArca,
		ScheduleAddr: "http://example.invalid",
		ArcaAPIKey:   "test-key",
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := newCloseNotifyRecorder()

	// When
	router.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), constants.HeaderEnvInstanceID) {
		t.Fatalf("body %q does not mention missing instance header", w.Body.String())
	}
}

func TestArcaProxyPortFromHeader_returnsPortOrDefault(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "uses explicit URL port", raw: "http://127.0.0.1:18080", want: 18080},
		{name: "uses explicit host port", raw: "127.0.0.1:19090", want: 19090},
		{name: "defaults when empty", raw: "", want: arcaProxyDefaultPort},
		{name: "defaults when port missing", raw: "http://127.0.0.1", want: arcaProxyDefaultPort},
		{name: "defaults when malformed", raw: "http://127.0.0.1:bad", want: arcaProxyDefaultPort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := arcaProxyPortFromHeader(tt.raw)

			// Then
			if got != tt.want {
				t.Fatalf("port = %d, want %d", got, tt.want)
			}
		})
	}
}

type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
	closeCh chan bool
}

func newCloseNotifyRecorder() *closeNotifyRecorder {
	return &closeNotifyRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeCh:          make(chan bool, 1),
	}
}

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	return r.closeCh
}
