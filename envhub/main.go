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
	"strconv"

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

	blobBackend string

	templateId  string
	callbackURL string
)

func init() {
	pflag.IntVar(&serverPort, "port", 8080, "Server port")
	pflag.IntVar(&metricsPort, "metrics-port", 9091, "Metrics port")
	pflag.StringVar(&storageBackend, "storage-backend", "redis", "Env storage backend (registered via service.RegisterEnvStorage)")
	pflag.StringVar(&redisAddr, "redis-addr", "localhost:6379", "Redis address")
	pflag.StringVar(&redisUsername, "redis-username", "", "Redis username")
	pflag.StringVar(&redisPassword, "redis-password", "", "Redis password")
	pflag.IntVar(&redisDB, "redis-db", 0, "Redis DB index")
	pflag.StringVar(&redisKeyPrefix, "redis-key-prefix", "env", "Redis key prefix for env data")
	pflag.StringVar(&blobBackend, "blob-backend", "oss", "Blob storage backend (oss or registered alternative). Empty disables blob features.")
	pflag.StringVar(&templateId, "template-id", "", "Template ID for pipeline or workflow (optional)")
	pflag.StringVar(&callbackURL, "callback-url", "", "Callback URL to notify after operation completion (optional)")
}

func main() {
	pflag.Parse()

	metrics := models.NewMetrics()
	metricsController := controller.NewMetricsController(metrics)

	envStorage, err := service.BuildEnvStorage(storageBackend, map[string]string{
		"addr":      redisAddr,
		"username":  redisUsername,
		"password":  redisPassword,
		"db":        strconv.Itoa(redisDB),
		"keyPrefix": redisKeyPrefix,
	})
	if err != nil {
		log.Fatalf("Failed to initialize env storage: %v", err)
	}

	healthChecker := controller.NewEnvStorageHealthChecker(envStorage)

	blobStorage, err := service.BuildBlobStorage(blobBackend, nil)
	if err != nil {
		log.Fatalf("Failed to initialize blob storage: %v", err)
	}
	if blobStorage == nil {
		log.Printf("Blob storage backend %q produced nil instance; blob features disabled", blobBackend)
	}

	var ciTrigger service.CITrigger
	if templateId != "" && callbackURL != "" {
		ciTrigger = clients.ACITrigger{
			TemplateId:  templateId,
			CallbackURL: callbackURL,
		}
	}

	envController := controller.NewEnvController(envStorage, blobStorage, ciTrigger)
	healthController := controller.NewHealthController(metrics, healthChecker)
	dataController := controller.NewDatasourceController()

	go func() {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(gin.Logger())
		r.Use(gin.Recovery())
		r.Use(middleware.MetricsMiddleware(metrics))
		r.Use(middleware.HealthCheckMiddleware(metrics, healthChecker))
		r.Use(middleware.TraceMiddleware())
		logger := middleware.InitLogger()
		defer func() {
			if err := logger.Sync(); err != nil {
				log.Printf("Failed to sync logger: %v", err)
			}
		}()
		r.Use(middleware.LoggingMiddleware(logger))

		envController.RegisterEnvRoutes(r)
		dataController.RegisterDataRoutes(r)

		r.GET("/health", healthController.Health)

		addr := fmt.Sprintf(":%d", serverPort)
		log.Printf("Starting main server on %s with storageBackend %s", addr, storageBackend)
		if err := r.Run(addr); err != nil {
			log.Fatalf("Failed to start main server: %v", err)
		}
	}()

	go func() {
		mr := gin.New()
		mr.Use(gin.Recovery())

		mr.GET("/metrics", metricsController.PrometheusHandler())

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

	select {}
}
