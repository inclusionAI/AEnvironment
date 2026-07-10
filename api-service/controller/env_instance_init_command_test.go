package controller

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCreateEnvInstanceRequest_bindsInitCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"envName":"swebench@1.0.4","init_command":"python3 -m http.server 8081"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/env-instance", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var req CreateEnvInstanceRequest
	err := c.ShouldBindJSON(&req)

	if err != nil {
		t.Fatalf("binding error: %v", err)
	}
	if req.InitCommand != "python3 -m http.server 8081" {
		t.Fatalf("init command = %q, want request value", req.InitCommand)
	}
}
