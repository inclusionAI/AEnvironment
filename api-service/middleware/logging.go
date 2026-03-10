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
	"api-service/constants"
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const maxBodyLogSize = 2048 // 2KB body truncation limit

// InitLogger initializes logrus with lumberjack log rotation.
// logPath: log file path, empty means default /home/admin/logs/api-service.log
// logLevel: log level string (debug, info, warn, error), empty means info
func InitLogger(logPath, logLevel string) {
	if logPath == "" {
		logPath = "/home/admin/logs/api-service.log"
	}

	lv, err := log.ParseLevel(logLevel)
	if err != nil {
		lv = log.InfoLevel
	}
	log.SetLevel(lv)

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
	})

	log.SetOutput(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    500, // megabytes
		MaxBackups: 10,
		MaxAge:     7, // days
		Compress:   false,
	})
}

// ResponseWriter is a custom ResponseWriter for capturing response body
type ResponseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write overrides Write method to capture response body
func (w ResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteString overrides WriteString method to capture response body
func (w ResponseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// LoggingMiddleware creates logging middleware using logrus
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Filter health check path, don't log
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		start := time.Now()

		// Read request body
		var reqBody []byte
		if c.Request.Body != nil {
			reqBody, _ = io.ReadAll(c.Request.Body)
			// Restore Body, because ReadAll consumes it
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		}

		// Create custom ResponseWriter to capture response
		blw := &ResponseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = blw

		// Continue processing request
		c.Next()

		// Calculate processing time
		latency := time.Since(start)

		// Get response status code
		statusCode := c.Writer.Status()

		// Build log fields
		fields := log.Fields{
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"status":    statusCode,
			"latency":   latency.String(),
			"client_ip": c.ClientIP(),
			"mcp_url":   c.Request.Header.Get(constants.HeaderMCPServerURL),
			"inst_id":   c.Request.Header.Get(constants.HeaderEnvInstanceID),
		}

		// Add request body (truncated)
		if len(reqBody) > 0 {
			fields["request_body"] = truncateString(string(reqBody), maxBodyLogSize)
		}

		// Add response body (truncated)
		if blw.body.Len() > 0 {
			fields["response_body"] = truncateString(blw.body.String(), maxBodyLogSize)
		}

		// Log error information (if any)
		if len(c.Errors) > 0 {
			fields["error"] = c.Errors.ByType(gin.ErrorTypePrivate).String()
		}

		entry := log.WithFields(fields)

		// Determine log level based on status code
		if statusCode >= 500 {
			entry.Error("API Error")
		} else if statusCode >= 400 {
			entry.Warn("API Warning")
		} else {
			entry.Info("API Access")
		}
	}
}

// truncateString truncates a string to maxLen bytes
func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "...(truncated)"
	}
	return s
}
