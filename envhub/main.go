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
	"envhub/clients"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"

	"envhub/controller"
	"envhub/middleware"
	"envhub/models"
	"envhub/service"
)

var (
	serverPort  int
	metricsPort int

	storageBackend string
	redisAddr      string
	redisUsername  string
	redisPassword  string
	redisDB        int
	redisKeyPrefix string

	templateId  string
	callbackURL string
)

func init() {
	pflag.IntVar(&serverPort, "port", 8080, "Server port")
	pflag.IntVar(&metricsPort, "metrics-port", 9091, "Metrics port")
	pflag.StringVar(&storageBackend, "storage-backend", "redis", "Env storage backend: redis")
	pflag.StringVar(&redisAddr, "redis-addr", "localhost:6379", "Redis address")
	pflag.StringVar(&redisUsername, "redis-username", "", "Redis username")
	pflag.StringVar(&redisPassword, "redis-password", "", "Redis password")
	pflag.IntVar(&redisDB, "redis-db", 0, "Redis DB index")
	pflag.StringVar(&redisKeyPrefix, "redis-key-prefix", "env", "Redis key prefix for env data")
	pflag.StringVar(&templateId, "template-id", "", "Template ID for pipeline or workflow (optional)")
	pflag.StringVar(&callbackURL, "callback-url", "", "Callback URL to notify after operation completion (optional)")
}

func main() {
	// Parse command line arguments
	pflag.Parse()

	// Initialize monitoring metrics
	metrics := models.NewMetrics()
	metricsController := controller.NewMetricsController(metrics)

	// Initialize storage service
	envStorage, err := newEnvStorage(storageBackend)
	if err != nil {
		log.Fatalf("Failed to initialize env storage: %v", err)
	}

	// Initialize health checker
	healthChecker := controller.NewEnvStorageHealthChecker(envStorage)

	// Initialize OSS storage with configuration (optional)
	ossConfig := service.LoadOssConfigFromEnv()
	ossStorage, err := service.NewOssStorage(ossConfig)
	if err != nil {
		log.Fatalf("Failed to initialize OSS storage: %v", err)
	}
	if ossStorage == nil {
		log.Printf("OSS storage is not configured, OSS-related features will be disabled")
	}

	var ciTrigger clients.ACITrigger
	if templateId != "" && callbackURL != "" {
		ciTrigger = clients.ACITrigger{
			TemplateId:  templateId,
			CallbackURL: callbackURL,
		}
	}

	// Initialize controllers
	envController := controller.NewEnvController(envStorage, ossStorage, ciTrigger)

	healthController := controller.NewHealthController(metrics, healthChecker)
	dataController := controller.NewDatasourceController()
	// TODO: TokenController needs to be updated to use Redis instead of MetaService
	// tokenController := controller.NewTokenController(service.NewTokenStorage())

	// Start main service
	go func() {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(gin.Logger())
		r.Use(gin.Recovery())
		r.Use(middleware.MetricsMiddleware(metrics))
		r.Use(middleware.HealthCheckMiddleware(metrics, healthChecker))
		r.Use(middleware.TraceMiddleware())
		// Initialize logger
		logger := middleware.InitLogger()
		defer func() {
			if err := logger.Sync(); err != nil {
				log.Printf("Failed to sync logger: %v", err)
			}
		}()
		r.Use(middleware.LoggingMiddleware(logger))

		// Register routes
		envController.RegisterEnvRoutes(r)
		dataController.RegisterDataRoutes(r)
		// TODO: Re-enable token routes when TokenStorage is migrated to Redis
		// tokenController.RegisterTokenRoutes(r)

		// Health check endpoint
		r.GET("/health", healthController.Health)

		// Start main server
		addr := fmt.Sprintf(":%d", serverPort)
		log.Printf("Starting main server on %s with storageBackend %s", addr, storageBackend)
		if err := r.Run(addr); err != nil {
			log.Fatalf("Failed to start main server: %v", err)
		}
	}()

	// Start metrics service
	go func() {
		mr := gin.New()
		mr.Use(gin.Recovery())

		// Prometheus metrics endpoint
		mr.GET("/metrics", metricsController.PrometheusHandler())

		// Metrics health check endpoint
		mr.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "metrics server healthy",
			})
		})

		metricsAddr := fmt.Sprintf(":%d", metricsPort)
		log.Printf("Starting metrics server on %s", metricsAddr)
		if err := mr.Run(metricsAddr); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()

	// Block main goroutine
	select {}
}

func newEnvStorage(backend string) (service.EnvStorage, error) {
	switch strings.ToLower(backend) {
	case "", "redis":
		return service.NewRedisEnvStorage(service.RedisEnvStorageOptions{
			Addr:      redisAddr,
			Username:  redisUsername,
			Password:  redisPassword,
			DB:        redisDB,
			KeyPrefix: redisKeyPrefix,
		})
	default:
		return nil, fmt.Errorf("unsupported storage backend %s, only redis is supported", backend)
	}
}
