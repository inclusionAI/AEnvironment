package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"envhub/models"
	"envhub/service"
)

type memoryEnvStorage struct {
	envs             map[string]*models.Env
	labels           map[string]map[string]string
	resourceVersions map[string]int64
}

func newMemoryEnvStorage() *memoryEnvStorage {
	return &memoryEnvStorage{
		envs:             map[string]*models.Env{},
		labels:           map[string]map[string]string{},
		resourceVersions: map[string]int64{},
	}
}

func (s *memoryEnvStorage) Get(_ context.Context, key string) (*models.Env, int64, error) {
	env, ok := s.envs[key]
	if !ok {
		return nil, 0, fmt.Errorf("env not found: %s", key)
	}
	return env, s.resourceVersions[key], nil
}

func (s *memoryEnvStorage) Create(_ context.Context, key string, env *models.Env, labels map[string]string) error {
	if _, ok := s.envs[key]; ok {
		return fmt.Errorf("env already exists: %s", key)
	}
	s.envs[key] = env
	s.labels[key] = labels
	s.resourceVersions[key] = 1
	return nil
}

func (s *memoryEnvStorage) Update(_ context.Context, key string, env *models.Env, resourceVersion int64, labels map[string]string) error {
	if _, ok := s.envs[key]; !ok {
		return fmt.Errorf("env not found: %s", key)
	}
	if s.resourceVersions[key] != resourceVersion {
		return fmt.Errorf("resource version mismatch for %s", key)
	}
	s.envs[key] = env
	s.labels[key] = labels
	s.resourceVersions[key]++
	return nil
}

func (s *memoryEnvStorage) Delete(_ context.Context, key string) error {
	delete(s.envs, key)
	delete(s.labels, key)
	delete(s.resourceVersions, key)
	return nil
}

func (s *memoryEnvStorage) List(_ context.Context, _ map[string]string) ([]string, error) {
	keys := make([]string, 0, len(s.envs))
	for key := range s.envs {
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *memoryEnvStorage) Watch(_ context.Context, _ int64, _ string, _ map[string]string) (service.WatchClient, error) {
	return nil, fmt.Errorf("watch is not supported")
}

type recordingTrigger struct {
	triggered chan *models.Env
}

func newRecordingTrigger() *recordingTrigger {
	return &recordingTrigger{triggered: make(chan *models.Env, 1)}
}

func (t *recordingTrigger) Trigger(env *models.Env) {
	t.triggered <- env
}

func Test_UpdateEnv_savesMetadataWithoutTriggeringBuild_whenBuildInputsUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := newMemoryEnvStorage()
	trigger := newRecordingTrigger()
	storage.envs["demo-1.0.0"] = models.NewEnv("demo-1.0.0", "demo", "", "1.0.0", "oss://demo")
	storage.envs["demo-1.0.0"].BuildConfig = map[string]interface{}{"dockerfile": "./Dockerfile"}
	storage.resourceVersions["demo-1.0.0"] = 1
	router := gin.New()
	NewEnvController(storage, nil, trigger).RegisterEnvRoutes(router)
	body := map[string]interface{}{
		"name":    "demo",
		"version": "1.0.0",
		"tags":    []string{"linux", "edited"},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPut, "/env/demo/1.0.0", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if got := storage.labels["demo-1.0.0"]["name"]; got != "demo" {
		t.Fatalf("expected label name demo, got %q", got)
	}
	select {
	case <-trigger.triggered:
		t.Fatalf("expected no build trigger for metadata-only update")
	default:
	}
}

func Test_UpdateEnv_triggersBuild_whenBuildConfigChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := newMemoryEnvStorage()
	trigger := newRecordingTrigger()
	storage.envs["demo-1.0.0"] = models.NewEnv("demo-1.0.0", "demo", "", "1.0.0", "oss://demo")
	storage.envs["demo-1.0.0"].BuildConfig = map[string]interface{}{"dockerfile": "./Dockerfile"}
	storage.resourceVersions["demo-1.0.0"] = 1
	router := gin.New()
	NewEnvController(storage, nil, trigger).RegisterEnvRoutes(router)
	body := map[string]interface{}{
		"name":        "demo",
		"version":     "1.0.0",
		"buildConfig": map[string]interface{}{"dockerfile": "./Containerfile"},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPut, "/env/demo/1.0.0", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	select {
	case env := <-trigger.triggered:
		if env.Name != "demo" {
			t.Fatalf("expected trigger env demo, got %q", env.Name)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected build trigger")
	}
}

func Test_CreateEnv_acceptsCollectionRouteWithoutTrailingSlash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := newMemoryEnvStorage()
	router := gin.New()
	NewEnvController(storage, nil, nil).RegisterEnvRoutes(router)
	body := map[string]interface{}{
		"name":        "demo",
		"description": "Demo env",
		"version":     "1.0.0",
		"status":      "Ready",
		"buildConfig": map[string]interface{}{"dockerfile": "./Dockerfile"},
		"testConfig":  map[string]interface{}{"script": ""},
		"deployConfig": map[string]interface{}{
			"cpu":    "1",
			"memory": "2Gi",
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/env", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	env, _, err := storage.Get(context.Background(), "demo-1.0.0")
	if err != nil {
		t.Fatalf("expected stored env: %v", err)
	}
	if env.Description != "Demo env" {
		t.Fatalf("expected description to persist, got %q", env.Description)
	}
}

func Test_ListEnvs_returnsEmptyArray_whenNoEnvsExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewEnvController(newMemoryEnvStorage(), nil, nil).RegisterEnvRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/env", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []models.Env `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Data == nil {
		t.Fatalf("expected empty array data, got nil")
	}
	if len(response.Data) != 0 {
		t.Fatalf("expected empty env list, got %d entries", len(response.Data))
	}
}
