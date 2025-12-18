// main.go
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
	"log"
	"net/http"
	"runtime"
	"time"

	"api-service/controller"
	"api-service/middleware"
	"api-service/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
)

var (
	scheduleAddr  string
	scheduleType  string
	backendAddr   string
	redisAddr     string
	redisPassword string
	qps           int64
	// New: token cache configuration
	tokenEnabled         bool
	tokenCacheMaxEntries int
	tokenCacheTTLMinutes int
	cleanupInterval      string
)

func init() {
	pflag.StringVar(&scheduleAddr, "schedule-addr", "", "Meta service address (host:port)")
	pflag.StringVar(&scheduleType, "schedule-type", "k8s", "sandbox service schedule type, currently only 'k8s', 'standard' support")
	pflag.StringVar(&backendAddr, "backend-addr", "", "backend service address (host:port)")

	pflag.Int64Var(&qps, "qps", int64(100), "total qps limit")
	pflag.BoolVar(&tokenEnabled, "token-enabled", false, "token validate enabled")
	pflag.IntVar(&tokenCacheMaxEntries, "token-cache-max-entries", 1000, "Maximum number of token cache entries (default 1000)")
	pflag.IntVar(&tokenCacheTTLMinutes, "token-cache-ttl-minutes", 1, "Token cache TTL in minutes (default 1)")

	pflag.StringVar(&redisAddr, "redis-addr", "", "Redis address (host:port)")
	pflag.StringVar(&redisPassword, "redis-password", "", "Redis password")
	pflag.StringVar(&cleanupInterval, "cleanup-interval", "5m", "Cleanup service interval (e.g., 5m, 1h)")
}

func healthChecker(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func main() {
	pflag.Parse()

	// Main routing engine
	gin.SetMode(gin.ReleaseMode)
	mainRouter := gin.Default()

	// Register global metrics middleware
	mainRouter.Use(middleware.MetricsMiddleware())

	// Initialize logger
	logger := middleware.InitLogger()
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}()
	mainRouter.Use(middleware.LoggingMiddleware(logger))
	// Main route configuration
	var redisClient *service.RedisClient = nil
	if redisAddr != "" {
		redisClient = service.InitRedis(redisAddr, redisPassword)
	}
	// Create BackendClient, pass cache configuration
	ttl := time.Duration(tokenCacheTTLMinutes) * time.Minute
	backendClient, err := service.NewBackendClient(backendAddr, tokenCacheMaxEntries, ttl)
	if err != nil {
		log.Fatalf("Failed to create backend client: %v", err)
	}

	var scheduleClient service.EnvInstanceService
	if scheduleType == "k8s" {
		scheduleClient = service.NewScheduleClient(scheduleAddr)
	} else if scheduleType == "standard" {
		scheduleClient = service.NewEnvInstanceClient(scheduleAddr)
	} else {
		log.Fatalf("unsupported schedule type: %v", scheduleType)
	}

	envInstanceController := controller.NewEnvInstanceController(scheduleClient, backendClient, redisClient)
	// Main route configuration
	mainRouter.POST("/env-instance",
		middleware.AuthTokenMiddleware(tokenEnabled, backendClient),
		middleware.InstanceLimitMiddleware(redisClient),
		middleware.RateLimit(qps),
		envInstanceController.CreateEnvInstance)
	mainRouter.GET("/env-instance/:id/list", middleware.AuthTokenMiddleware(tokenEnabled, backendClient), envInstanceController.ListEnvInstances)
	mainRouter.GET("/env-instance/:id", middleware.AuthTokenMiddleware(tokenEnabled, backendClient), envInstanceController.GetEnvInstance)
	mainRouter.DELETE("/env-instance/:id", middleware.AuthTokenMiddleware(tokenEnabled, backendClient), envInstanceController.DeleteEnvInstance)
	mainRouter.GET("/health", healthChecker)
	mainRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// MCP dedicated routing engine
	mcpRouter := gin.Default()
	mcpGroup := mcpRouter.Group("/")
	controller.NewMCPGateway(mcpGroup)

	// Start two services
	go func() {
		port := ":8080"
		if runtime.GOOS != "linux" {
			port = ":8070"
		}
		if err := mainRouter.Run(port); err != nil {
			log.Fatalf("Failed to start main server: %v", err)
		}
	}()

	go func() {
		if err := mcpRouter.Run(":8081"); err != nil {
			log.Fatalf("Failed to start MCP server: %v", err)
		}
	}()

	// clean expired env instance
	interval, err := time.ParseDuration(cleanupInterval)
	if err != nil {
		log.Fatalf("Invalid cleanup interval: %v", err)
	}
	cleanManager := service.NewAEnvCleanManager(service.NewKubeCleaner(scheduleClient), interval)
	go cleanManager.Start()
	defer cleanManager.Stop()

	// Block main goroutine
	select {}
}
