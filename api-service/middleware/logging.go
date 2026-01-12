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
	"bytes"
	"io"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger initializes zap logger with log rotation
func InitLogger(logPath string) *zap.Logger {
	if logPath == "" {
		logPath = "/home/admin/logs/aenvcore-api-service.log"
	}
	// Console encoder
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	// File encoder (JSON format)
	fileEncoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	// Lumberjack log rotation configuration
	logWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logPath, // Log file path
		MaxSize:    100,     // Maximum size of each log file (MB)
		MaxBackups: 30,      // Maximum number of old files to retain
		MaxAge:     0,       // Maximum age of old files in days (0 means permanent)
		Compress:   false,   // Whether to compress old files
	})
	// Console output (stdout)
	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
	consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleCore := zapcore.NewCore(consoleEncoder, consoleDebugging, zapcore.DebugLevel)
	// File output
	fileCore := zapcore.NewCore(fileEncoder, logWriter, zapcore.DebugLevel)
	// Merge multiple cores (dual write)
	core := zapcore.NewTee(consoleCore, fileCore)
	logger := zap.New(core, zap.AddCaller(), zap.Development())
	return logger
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

// LoggingMiddleware creates logging middleware
func LoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
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

		// Log
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		}

		// Add request body (if exists)
		if len(reqBody) > 0 {
			fields = append(fields, zap.String("request_body", string(reqBody)))
		}

		// Add response body (if exists)
		if blw.body.Len() > 0 {
			fields = append(fields, zap.String("response_body", blw.body.String()))
		}

		// Log error information (if any)
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.ByType(gin.ErrorTypePrivate).String()))
		}

		// Determine log level based on status code
		if statusCode >= 500 {
			logger.Error("API Error", fields...)
		} else if statusCode >= 400 {
			logger.Warn("API Warning", fields...)
		} else {
			logger.Info("API Access", fields...)
		}
	}
}
