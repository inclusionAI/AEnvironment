/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	dockerserver "docker-controller/pkg/docker_http_server"

	"k8s.io/klog"
)

const (
	dockerRepoName = "aenv-docker-controller"
)

var (
	dockerServerPort int
)

func main() {
	klog.Infof("entering main for AEnv Docker controller server")

	flag.IntVar(&dockerServerPort, "server-port", 8080, "The value for server port.")
	klog.InitFlags(nil)
	flag.Parse()

	dockerStartHttpServer()
}

func dockerStartHttpServer() {
	klog.Infof("starting AENV Docker controller http server...")

	// AENV Container Manager
	aenvContainerManager, err := dockerserver.NewAEnvContainerHandler()
	if err != nil {
		klog.Fatalf("failed to create AENV Container manager, err is %v", err)
	}

	// Set up routes
	mux := http.NewServeMux()

	mux.Handle("/pods", aenvContainerManager)
	mux.Handle("/pods/", aenvContainerManager)

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Readiness endpoint
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Start server
	poolserver := &http.Server{
		Addr:         fmt.Sprintf(":%d", dockerServerPort),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	klog.Infof("AEnv Docker controller server starts, listening on port: %d", dockerServerPort)
	if err := poolserver.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("AEnv Docker controller server failed to start, err is %v", err)
	}
}

