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

package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceMiddleware generates TraceID for requests and records request latency
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate Trace ID
		traceID := uuid.New().String()
		c.Set("trace_id", traceID)

		// Record start time
		start := time.Now()

		fmt.Printf("[TraceID: %s] [%s] %s %s\n",
			traceID,
			start.Format("2006-01-02 15:04:05"),
			c.Request.Method,
			c.Request.URL.Path,
		)

		// Process request
		c.Next()

		// Calculate latency after request processing completes
		latency := time.Since(start)
		statusCode := c.Writer.Status()

		fmt.Printf("[TraceID: %s] [%s] %s %s | Status: %d | Latency: %v\n",
			traceID,
			time.Now().Format("2006-01-02 15:04:05"),
			c.Request.Method,
			c.Request.URL.Path,
			statusCode,
			latency,
		)
	}
}
