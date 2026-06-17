package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func Test_RegisterConsoleRoutes_servesIndexForConsoleRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerConsoleRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/console", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "EnvHub Console") {
		t.Fatalf("expected console index body, got %q", recorder.Body.String())
	}
}

func Test_RegisterConsoleRoutes_servesIndexForSpaRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerConsoleRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/console/env/demo/1.0.0", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "EnvHub Console") {
		t.Fatalf("expected console index body, got %q", recorder.Body.String())
	}
}

func Test_RegisterConsoleRoutes_returnsNotFoundForMissingAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerConsoleRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/console/assets/missing.js", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}
