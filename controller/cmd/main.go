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
	_ "net/http/pprof"
	"os"
	"sync/atomic"
	"time"

	aenvhubserver "controller/pkg/aenvhub_http_server"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

const (
	repoName = "aenv-controller"
)

var (
	defaultNamespace string
	logDir           string
	serverPort       int

	controllerManager manager.Manager
)

func main() {
	klog.Infof("entering main for AEnv server")

	flag.StringVar(&defaultNamespace, "namespace", "aenvsandbox", "The namespace that pods are using.")
	flag.StringVar(&logDir, "logdir", "/home/admin/logs", "The dir of log output.")
	flag.IntVar(&serverPort, "server-port", 8080, "The value for server port.")
	klog.InitFlags(nil)

	// SetUpController() -> AddReadiness() -> Provide StartHttpServer() service after leader election.
	SetUpController()
}

func StartHttpServer() {

	klog.Infof("starting AENV http server...")

	// AENV Pod Manager
	aenvPodManager, err := aenvhubserver.NewAEnvPodHandler()
	if err != nil {
		klog.Fatalf("failed to create AENV Pod manager, err is %v", err)
	}

	// Set up routes
	mux := http.NewServeMux()

	mux.Handle("/pods", aenvPodManager)
	mux.Handle("/pods/", aenvPodManager)

	// Start server
	poolserver := &http.Server{
		Addr:         fmt.Sprintf(":%d", serverPort),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	klog.Infof("AEnv server starts, listening on port: %d", serverPort)
	if err := poolserver.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("AEnv server failed to start, err is %v", err)
	}
}

func SetUpController() {
	var (
		metricsAddr string
		pprofAddr   string
		qps         int
		burst       int

		enableLeaderElection                                          bool
		leaderDuration, leaderRenewDuration, leaderRetryPeriodDuation string
	)
	flag.StringVar(&metricsAddr, "metrics-addr", ":8088", "The address the metric endpoint binds to.")
	flag.StringVar(&pprofAddr, "pprof-addr", ":8089", "The address the pprof endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true, "Enable leader election")
	flag.StringVar(&leaderDuration, "leader-elect-lease-duration", "65s", "leader election lease duration")
	flag.StringVar(&leaderRenewDuration, "leader-elect-renew-deadline", "60s", "leader election renew deadline")
	flag.StringVar(&leaderRetryPeriodDuation, "leader-elect-retry-period", "2s", "leader election retry period")
	flag.IntVar(&qps, "qps", 50, "QPS for kubernetes clientset config.")
	flag.IntVar(&burst, "burst", 100, "Burst for kubernetes clienset config.")

	flag.Parse()

	leaseTime, err := time.ParseDuration(leaderDuration)
	if err != nil {
		klog.Error(err, "unable to parse leaseDuration", "leaderDuration", leaderDuration)
		os.Exit(1)
	}
	leaseRenewTime, err := time.ParseDuration(leaderRenewDuration)
	if err != nil {
		klog.Error(err, "unable to parse leaseRenewDuration", "leaseRenewDuration", leaderRenewDuration)
		os.Exit(1)
	}
	leaderRetryPeriodTIme, err := time.ParseDuration(leaderRetryPeriodDuation)
	if err != nil {
		klog.Error(err, "unable to parse leaseRenewDuration", "leaderRetryPeriod", leaderRetryPeriodDuation)
		os.Exit(1)
	}

	go func() {
		klog.Infof("starting pprof http server, listening on %s", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			klog.Errorf("error starting pprof http server, err is %v", err)
		}
	}()

	// Get a config to talk to the apiserver
	klog.Infof("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		klog.Errorf("unable to set up client config, err is %v", err)
		os.Exit(1)
	}
	cfg.QPS = float32(qps)
	cfg.Burst = burst
	cfg.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	cfg.UserAgent = "aenv-controller"

	// Create a new Cmd to provide shared dependencies and start components
	klog.Infof("setting up manager")
	controllerManager, err = manager.New(cfg, manager.Options{
		MetricsBindAddress:      metricsAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        fmt.Sprintf("%s-leader", repoName),
		LeaderElectionNamespace: defaultNamespace,
		LeaseDuration:           &leaseTime,
		RenewDeadline:           &leaseRenewTime,
		RetryPeriod:             &leaderRetryPeriodTIme,
	})

	if err != nil {
		klog.Errorf("unable to set up overall controller manager, err is %v", err)
		os.Exit(1)
	}

	klog.Infof("Registering Components.")

	// Setup Scheme for all resources
	klog.Infof("setting up scheme")
	// utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	if err = clientgoscheme.AddToScheme(controllerManager.GetScheme()); err != nil {
		klog.Errorf("unable add APIs to scheme, err is %v", err)
		os.Exit(1)
	}

	// Setup all Controllers
	klog.Infof("Setting up controller")
	if err = AddToManager(controllerManager); err != nil {
		klog.Errorf("unable to register controllers to the manager, err is %v", err)
		os.Exit(1)
	}

	AddReadiness(controllerManager)

	// Start the Cmd
	klog.Infof("Starting the Cmd.")
	if err = controllerManager.Start(signals.SetupSignalHandler()); err != nil {
		klog.Errorf("unable to run the manager, err is %v", err)
		os.Exit(1)
	}
}

func AddReadiness(mgr manager.Manager) {

	// Record leader status
	var isLeader atomic.Bool
	// Listen to mgr.Elected(), set flag to true when becoming leader
	go func() {
		<-mgr.Elected() // When closed, it means leader has been acquired
		isLeader.Store(true)

		klog.Infof("This controller is now the leader")

		StartHttpServer()
	}()

	// readiness API
	readyzHandler := func(w http.ResponseWriter, r *http.Request) {
		if isLeader.Load() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("not leader"))
		}
	}
	// starts readiness server
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/readyz", readyzHandler)
		srv := &http.Server{
			Addr:    ":8081",
			Handler: mux,
		}
		klog.Infof("Readiness server started on 8081")
		if err := srv.ListenAndServe(); err != nil {
			klog.Errorf("server error: %v\n", err)
		}
	}()
}

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs = map[string]func(manager.Manager) error{}

// AddToManager adds all Controllers to the Manager
// Automatically generate RBAC rules to allow the Controller to leader election
// Automatically generate RBAC rules to allow the Controller to create events
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create
func AddToManager(m manager.Manager) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m); err != nil {
			return err
		}
	}
	return nil
}
