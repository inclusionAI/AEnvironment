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
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"

	"api-service/controller"
	"api-service/metrics"
	"api-service/middleware"
	"api-service/service"

	"github.com/gin-contrib/pprof"
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
	// Supported engines: arca.
	arcaBaseURL string
	// Supported engines: arca.
	arcaAPIKey string
)

func init() {
	pflag.StringVar(&scheduleAddr, "schedule-addr", "", "Meta service address (host:port)")
	pflag.StringVar(&scheduleType, "schedule-type", "k8s", "sandbox service schedule type: 'k8s', 'standard', 'faas', or 'arca'")
	pflag.StringVar(&backendAddr, "backend-addr", "", "backend service address (host:port)")

	pflag.Int64Var(&qps, "qps", int64(100), "total qps limit")
	pflag.BoolVar(&tokenEnabled, "token-enabled", false, "token validate enabled")
	pflag.IntVar(&tokenCacheMaxEntries, "token-cache-max-entries", 1000, "Maximum number of token cache entries (default 1000)")
	pflag.IntVar(&tokenCacheTTLMinutes, "token-cache-ttl-minutes", 1, "Token cache TTL in minutes (default 1)")

	pflag.StringVar(&redisAddr, "redis-addr", "", "Redis address (host:port)")
	pflag.StringVar(&redisPassword, "redis-password", "", "Redis password")
	pflag.StringVar(&cleanupInterval, "cleanup-interval", "5m", "Cleanup service interval (e.g., 5m, 1h)")

	// Arca sandbox engine flags. Supported engines: arca.
	pflag.StringVar(&arcaBaseURL, "arca-base-url", "", "Arca sandbox OpenAPI base URL. Supported engines: arca")
	pflag.StringVar(&arcaAPIKey, "arca-api-key", "", "Arca sandbox OpenAPI key; falls back to ARCA_API_KEY env. Supported engines: arca")
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

	// Initialize logger (logrus + lumberjack)
	middleware.InitLogger("", "info")
	mainRouter.Use(middleware.LoggingMiddleware())
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
	var envServiceController *controller.EnvServiceController
	switch scheduleType {
	case "k8s":
		scheduleClient = service.NewScheduleClient(scheduleAddr)
		envServiceController = controller.NewEnvServiceController(scheduleClient, backendClient, redisClient)
	case "standard":
		scheduleClient = service.NewEnvInstanceClient(scheduleAddr)
	case "faas":
		scheduleClient = service.NewFaaSClient(scheduleAddr)
	case "arca":
		// Supported engines: arca.
		key := arcaAPIKey
		if key == "" {
			key = os.Getenv("ARCA_API_KEY")
		}
		if arcaBaseURL == "" {
			log.Fatalf("--arca-base-url is required when --schedule-type=arca")
		}
		if key == "" {
			log.Fatalf("arca API key missing: set --arca-api-key or ARCA_API_KEY")
		}
		scheduleClient = service.NewArcaClient(arcaBaseURL, key)
	default:
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
	mainRouter.POST("/env-instance/:id/presign-url", middleware.AuthTokenMiddleware(tokenEnabled, backendClient), envInstanceController.PresignURL)

	// Service routes
	if envServiceController != nil {
		mainRouter.POST("/env-service",
			middleware.AuthTokenMiddleware(tokenEnabled, backendClient),
			middleware.RateLimit(qps),
			envServiceController.CreateEnvService)
		mainRouter.GET("/env-service/:id/list", middleware.AuthTokenMiddleware(tokenEnabled, backendClient), envServiceController.ListEnvServices)
		mainRouter.GET("/env-service/:id", middleware.AuthTokenMiddleware(tokenEnabled, backendClient), envServiceController.GetEnvService)
		mainRouter.DELETE("/env-service/:id", middleware.AuthTokenMiddleware(tokenEnabled, backendClient), envServiceController.DeleteEnvService)
		mainRouter.PUT("/env-service/:id", middleware.AuthTokenMiddleware(tokenEnabled, backendClient), envServiceController.UpdateEnvService)
	}

	mainRouter.GET("/health", healthChecker)
	mainRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))
	pprof.Register(mainRouter)

	// MCP dedicated routing engine
	// Note: MCP uses the same logrus global logger (writes to same log file)
	// since logrus is a global singleton. For separate MCP log files,
	// use a dedicated logrus instance in the future.
	mcpRouter := gin.Default()
	mcpRouter.Use(middleware.MCPMetricsMiddleware())
	mcpRouter.Use(middleware.LoggingMiddleware())
	mcpGroup := mcpRouter.Group("/")
	controller.NewMCPGateway(mcpGroup, controller.MCPGatewayConfig{
		ScheduleType: scheduleType,
		ArcaBaseURL:  arcaBaseURL,
		ArcaAPIKey:   arcaAPIKey,
	})

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
	cleanManager := service.NewAEnvCleanManager(scheduleClient, interval).
		WithMetrics(middleware.IncrementCleanupSuccess, middleware.IncrementCleanupFailure)

	// Start a unified periodic task that shares a single ListEnvInstances call
	// across cleanup and metrics collection, reducing redundant requests to meta-service.
	if scheduleType == "faas" {
		if faasClient, ok := scheduleClient.(*service.FaaSClient); ok {
			metricsCollector := metrics.NewCollector(faasClient, interval)
			go startUnifiedPeriodicTask(scheduleClient, cleanManager, metricsCollector, interval)
		} else {
			go cleanManager.Start()
		}
	} else {
		go cleanManager.Start()
	}

	// Block main goroutine
	select {}
}

// startUnifiedPeriodicTask runs cleanup and metrics collection in a single ticker loop,
// sharing one ListEnvInstances call per cycle. A random jitter at startup disperses
// the tick phase across multiple api-service replicas to avoid thundering herd.
func startUnifiedPeriodicTask(
	envInstanceService service.EnvInstanceService,
	cleanManager *service.AEnvCleanManager,
	metricsCollector *metrics.Collector,
	interval time.Duration,
) {
	// Random jitter to stagger tickers across replicas
	jitter := time.Duration(rand.Int63n(int64(interval)))
	log.Infof("Unified periodic task: starting after jitter %v (interval %v)", jitter, interval)
	time.Sleep(jitter)

	runOnce := func() {
		envInstances, err := envInstanceService.ListEnvInstances("")
		if err != nil {
			log.Warnf("Unified periodic task: failed to list instances: %v", err)
			return
		}

		// Feed the same data to both consumers
		cleanManager.CleanupFromInstances(envInstances)
		metricsCollector.CollectFromEnvInstances(envInstances)
	}

	runOnce()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		runOnce()
	}
}
