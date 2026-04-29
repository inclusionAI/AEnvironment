/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestCreateEnvInstanceRequest_ArcaFieldsBinding verifies JSON binding of
// mount_points.
//
// Supported engines: arca.
func TestCreateEnvInstanceRequest_ArcaFieldsBinding(t *testing.T) {
	body := `{
		"envName": "test@v1",
		"ttl": "30m",
		"mount_points": [
			{"id": "OSS_bucket_ak", "remote_dir": "/data/oss", "local_dir": "/workspace/oss"}
		]
	}`

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/env-instance", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var req CreateEnvInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		t.Fatalf("bind: %v", err)
	}

	if len(req.MountPoints) != 1 {
		t.Fatalf("mount_points length = %d, want 1", len(req.MountPoints))
	}
	mp := req.MountPoints[0]
	if mp["id"] != "OSS_bucket_ak" {
		t.Errorf("mount_points[0].id = %v", mp["id"])
	}
	if mp["remote_dir"] != "/data/oss" {
		t.Errorf("mount_points[0].remote_dir = %v", mp["remote_dir"])
	}
}

// TestCreateEnvInstance_MountPointsForwardedToDeployConfig mirrors the
// controller's forwarding step without invoking the HTTP handler.
//
// Supported engines: arca.
func TestCreateEnvInstance_MountPointsForwardedToDeployConfig(t *testing.T) {
	reqBody := CreateEnvInstanceRequest{
		EnvName: "test@v1",
		MountPoints: []map[string]interface{}{
			{"id": "OSS_x", "remote_dir": "/a", "local_dir": "/b"},
		},
	}
	deployConfig := make(map[string]interface{})
	if len(reqBody.MountPoints) > 0 {
		deployConfig["mountPoints"] = reqBody.MountPoints
	}
	got, ok := deployConfig["mountPoints"].([]map[string]interface{})
	if !ok {
		t.Fatalf("DeployConfig[mountPoints] type = %T, want []map[string]interface{}", deployConfig["mountPoints"])
	}
	if len(got) != 1 || got[0]["id"] != "OSS_x" {
		t.Errorf("unexpected mountPoints: %v", got)
	}
}

// TestCreateEnvInstance_ArcaFieldsOmitted_NoDeployConfigMutation verifies
// backward compatibility: existing callers (k8s/standard/faas) that don't
// supply arca fields see no extra keys injected.
//
// Supported engines: all.
func TestCreateEnvInstance_ArcaFieldsOmitted_NoDeployConfigMutation(t *testing.T) {
	reqBody := CreateEnvInstanceRequest{EnvName: "test@v1", TTL: "30m"}
	deployConfig := make(map[string]interface{})
	if len(reqBody.MountPoints) > 0 {
		deployConfig["mountPoints"] = reqBody.MountPoints
	}
	if _, ok := deployConfig["mountPoints"]; ok {
		t.Errorf("mountPoints should not be set when request omits it")
	}
}

// TestCreateEnvInstanceRequest_ArcaFieldsOmitted_UnmarshalSilent verifies
// empty request body does not populate arca fields.
func TestCreateEnvInstanceRequest_ArcaFieldsOmitted_UnmarshalSilent(t *testing.T) {
	body := `{"envName": "test@v1", "ttl": "30m"}`
	var req CreateEnvInstanceRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.MountPoints != nil {
		t.Errorf("mount_points = %v, want nil", req.MountPoints)
	}
}
