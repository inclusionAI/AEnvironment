package controller

import "testing"

func TestDatasourceImageName_returnsDatasource_whenDatasourceIsFullImage(t *testing.T) {
	deployConfig := map[string]interface{}{
		"imagePrefix": "registry.example.com/aenv/base",
	}

	got := datasourceImageName(deployConfig, "swebench/sweb.eval.case:latest")

	if got != "swebench/sweb.eval.case:latest" {
		t.Fatalf("image = %q, want full datasource image", got)
	}
}

func TestDatasourceImageName_usesImagePrefix_whenDatasourceIsTagSuffix(t *testing.T) {
	deployConfig := map[string]interface{}{
		"imagePrefix": "registry.example.com/aenv/base",
	}

	got := datasourceImageName(deployConfig, "case-123")

	if got != "registry.example.com/aenv/base:case-123" {
		t.Fatalf("image = %q, want prefixed datasource image", got)
	}
}

func TestDatasourceImageName_treatsTaggedLibraryImageAsFullImage(t *testing.T) {
	got := datasourceImageName(nil, "python:3.11")

	if got != "python:3.11" {
		t.Fatalf("image = %q, want tagged datasource image", got)
	}
}
