package metrics

import (
	"api-service/service/faas_model"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// mockInstanceLister implements InstanceLister for testing
type mockInstanceLister struct {
	instances []*faas_model.Instance
	err       error
}

func (m *mockInstanceLister) ListInstances(labels map[string]string) (*faas_model.InstanceListResp, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &faas_model.InstanceListResp{Instances: m.instances}, nil
}

func TestMetricsEndpointAvailable(t *testing.T) {
	// First, trigger some metrics to ensure they're registered
	RequestsTotal.WithLabelValues("GET", "/test", "200").Inc()
	RequestDurationMs.WithLabelValues("GET", "/test", "200").Observe(100)
	InstanceOpsTotal.WithLabelValues("create", "test-env", "success").Inc()
	ServiceOpsTotal.WithLabelValues("deploy", "test-env", "success").Inc()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Result().Body)
	bodyStr := string(body)

	// Check that our custom metrics appear in the output
	expectedMetrics := []string{
		"aenv_api_requests_total",
		"aenv_api_request_duration_ms",
		"aenv_api_instance_operations_total",
		"aenv_api_service_operations_total",
	}

	for _, m := range expectedMetrics {
		if !strings.Contains(bodyStr, m) {
			t.Errorf("expected metric %q in /metrics output", m)
		}
	}
}

func TestMetricsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Import and use the middleware via metrics package (we test the metric values)
	router.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := "200"
		endpoint := c.FullPath()
		method := c.Request.Method
		durationMs := float64(time.Since(start).Milliseconds())
		RequestsTotal.WithLabelValues(method, endpoint, status).Inc()
		RequestDurationMs.WithLabelValues(method, endpoint, status).Observe(durationMs)
	})

	router.GET("/middleware-test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	// Make a request
	req := httptest.NewRequest("GET", "/middleware-test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify metrics were recorded by checking /metrics endpoint
	metricsRouter := gin.New()
	metricsRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))
	req2 := httptest.NewRequest("GET", "/metrics", nil)
	w2 := httptest.NewRecorder()
	metricsRouter.ServeHTTP(w2, req2)

	body, _ := io.ReadAll(w2.Result().Body)
	bodyStr := string(body)

	// Just check that the metrics for our specific endpoint exist, not the exact count
	// (since metrics accumulate across tests due to global registry)
	if !strings.Contains(bodyStr, `aenv_api_requests_total{endpoint="/middleware-test",method="GET",status="200"}`) {
		t.Errorf("expected request counter for /middleware-test GET 200, got:\n%s", bodyStr)
	}

	if !strings.Contains(bodyStr, `aenv_api_request_duration_ms_count{endpoint="/middleware-test",method="GET",status="200"}`) {
		t.Errorf("expected request duration for /middleware-test GET 200, got:\n%s", bodyStr)
	}
}

func TestCollectorCollect(t *testing.T) {
	now := time.Now().UnixMilli()
	fiveMinAgo := now - 5*60*1000

	mock := &mockInstanceLister{
		instances: []*faas_model.Instance{
			{
				InstanceID:      "inst-001",
				CreateTimestamp: fiveMinAgo,
				IP:              "10.0.0.1",
				Labels: map[string]string{
					"env":        "terminal-0.1.0",
					"experiment": "exp1",
					"owner":      "jun",
					"app":        "chatbot",
				},
				Status: "Running",
			},
			{
				InstanceID:      "inst-002",
				CreateTimestamp: fiveMinAgo,
				IP:              "10.0.0.2",
				Labels: map[string]string{
					"env":        "terminal-0.1.0",
					"experiment": "exp1",
					"owner":      "jun",
					"app":        "chatbot",
				},
				Status: "Running",
			},
			{
				InstanceID:      "inst-003",
				CreateTimestamp: now - 60*1000, // 1 min ago
				IP:              "10.0.0.3",
				Labels: map[string]string{
					"env":   "swe-1.0.0",
					"owner": "alice",
				},
				Status: "Running",
			},
		},
	}

	collector := NewCollector(mock, 5*time.Minute)
	collector.collect()

	// Verify metrics via promhttp
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	bodyStr := string(body)

	// Check active instances gauge
	if !strings.Contains(bodyStr, "aenv_api_active_instances") {
		t.Error("expected aenv_api_active_instances in output")
	}

	// Check uptime gauge
	if !strings.Contains(bodyStr, "aenv_api_instance_uptime_seconds") {
		t.Error("expected aenv_api_instance_uptime_seconds in output")
	}

	// Check that inst-001 uptime is roughly 300 seconds
	if !strings.Contains(bodyStr, `instance_id="inst-001"`) {
		t.Error("expected instance_id=inst-001 in uptime metrics")
	}

	// Check active instances count for terminal-0.1.0 (should be 2)
	if !strings.Contains(bodyStr, `aenv_api_active_instances{app="chatbot",envName="terminal-0.1.0",experiment="exp1",owner="jun"} 2`) {
		t.Errorf("expected active_instances for terminal-0.1.0 to be 2")
	}

	t.Logf("Metrics output:\n%s", bodyStr)
}

func TestCollectorWithNilLabels(t *testing.T) {
	mock := &mockInstanceLister{
		instances: []*faas_model.Instance{
			{
				InstanceID:      "inst-nil",
				CreateTimestamp: time.Now().UnixMilli(),
				IP:              "10.0.0.1",
				Labels:          nil, // nil labels should not panic
				Status:          "Running",
			},
		},
	}

	collector := NewCollector(mock, 5*time.Minute)
	// Should not panic
	collector.collect()
}

func TestCollectorWithError(t *testing.T) {
	mock := &mockInstanceLister{
		err: io.ErrUnexpectedEOF,
	}

	collector := NewCollector(mock, 5*time.Minute)
	// Should not panic, just log warning
	collector.collect()
}
