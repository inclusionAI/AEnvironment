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

package controller

import (
	"api-service/constants"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// MCP gateway constant definitions
const (
	// Request header constants

	HeaderContentType  = "Content-Type"
	HeaderCacheControl = "Cache-Control"
	HeaderConnection   = "Connection"
	HeaderAccept       = "Accept"

	// Response content types
	ContentTypeSSE  = "text/event-stream"
	ContentTypeJSON = "application/json"

	// Path constants
	PathHealth = "/health"
	PathSSE    = "/sse"

	// HTTP methods
	MethodGET  = "GET"
	MethodPOST = "POST"

	// Schedule types (kept in sync with main.go --schedule-type).
	scheduleTypeArca = "arca"

	// Arca gateway conventions (kept in sync with service/arca_client.go).
	arcaGatewaySandboxPrefix = "/arca/api/v1/sandbox"
	arcaHeaderAPIKey         = "x-agent-sandbox-api-key"
	arcaHeaderSandboxID      = "x-agent-sandbox-id"
)

// MCPGatewayConfig is injected at api-service startup so the gateway never
// has to query per-request engine metadata.
type MCPGatewayConfig struct {
	ScheduleType string // "k8s" | "standard" | "faas" | "arca"
	ArcaBaseURL  string
	ArcaAPIKey   string
}

// MCPGateway MCP gateway struct
type MCPGateway struct {
	router    *gin.RouterGroup
	transport *http.Transport
	config    MCPGatewayConfig
}

// NewMCPGateway creates a new MCP gateway instance
func NewMCPGateway(router *gin.RouterGroup, cfg MCPGatewayConfig) *MCPGateway {
	gateway := &MCPGateway{
		router: router,
		transport: &http.Transport{
			MaxIdleConns:        2000,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		config: cfg,
	}

	gateway.setupRoutes()
	return gateway
}

// setupRoutes sets up routes
func (g *MCPGateway) setupRoutes() {
	g.router.Any("/*path", g.innerRouter)
}

func (g *MCPGateway) innerRouter(c *gin.Context) {
	path := c.Param("path")
	if g.config.ScheduleType == scheduleTypeArca {
		// Arca engine: the gateway doesn't trust SDK-provided proxy URL.
		// Target is derived from startup config + sandbox id header.
		if path == PathHealth {
			g.arcaHealthCheck(c)
			return
		}
		switch path {
		case PathSSE:
			g.handleArcaSSE(c)
		default:
			g.handleArcaHTTP(c)
		}
		return
	}

	proxyURL, _ := g.getMCPSeverURL(c)
	if proxyURL != "" {
		switch path {
		case PathSSE:
			g.handleMCPSSEWithHeader(c)
			return
		default:
			g.handleMCPHTTPWithHeader(c)
		}
	} else if path == PathHealth {
		g.healthCheck(c)
		return
	}
}

// healthCheck health check endpoint
func (g *MCPGateway) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "MCP Gateway is running",
	})
}

// arcaHealthCheck returns an aenv-envelope shaped response so the SDK's
// _wait_for_healthy (which expects success=true with data.status=healthy)
// can parse it. In arca mode the sandbox liveness is already guaranteed by
// the control-plane RUNNING status, so the gateway short-circuits here
// instead of round-tripping through the Arca gateway.
//
// Supported engines: arca.
func (g *MCPGateway) arcaHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"code":    0,
		"message": "",
		"data":    gin.H{"status": "healthy"},
	})
}

// getMCPSeverURL gets MCP server URL from request header
func (g *MCPGateway) getMCPSeverURL(c *gin.Context) (string, error) {
	mcpServerURL := c.GetHeader(constants.HeaderMCPServerURL)
	if mcpServerURL == "" {
		return "", &MCPError{
			Code:    http.StatusBadRequest,
			Message: constants.HeaderMCPServerURL + " header is required",
		}
	}
	return mcpServerURL, nil
}

// validateAndParseURL validates and parses URL
func (g *MCPGateway) validateAndParseURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, &MCPError{
			Code:    http.StatusBadRequest,
			Message: "Invalid MCP server URL format",
			Details: err.Error(),
		}
	}
	return parsedURL, nil
}

// buildTargetURL builds target URL
func (g *MCPGateway) buildTargetURL(serverURL *url.URL, path string, rawQuery string) url.URL {
	targetURL := *serverURL
	targetURL.Path = path
	if targetURL.Path == "" {
		targetURL.Path = "/"
	}
	targetURL.RawQuery = rawQuery
	return targetURL
}

// copyHeadersExcept copies request headers, excluding specified headers
func (g *MCPGateway) copyHeadersExcept(source http.Header, target http.Header, excludeHeaders ...string) {
	excludeMap := make(map[string]bool)
	for _, header := range excludeHeaders {
		excludeMap[header] = true
	}

	for name, values := range source {
		if excludeMap[name] {
			continue
		}
		for _, value := range values {
			target.Add(name, value)
		}
	}
}

// handleMCPSSEWithHeader handles MCP SSE connection (server address specified via HEADER)
func (g *MCPGateway) handleMCPSSEWithHeader(c *gin.Context) {
	// Get MCP server address from HEADER
	mcpServerURL, err := g.getMCPSeverURL(c)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			c.JSON(mcpErr.Code, gin.H{"error": mcpErr.Message})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	// Validate URL format
	serverURL, err := g.validateAndParseURL(mcpServerURL)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			c.JSON(mcpErr.Code, gin.H{"error": mcpErr.Message})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	// Set SSE response headers
	c.Header(HeaderContentType, ContentTypeSSE)
	c.Header(HeaderCacheControl, "no-cache")
	c.Header(HeaderConnection, "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	targetURL := g.buildTargetURL(serverURL, c.Request.URL.Path, c.Request.URL.RawQuery)

	// Create request to MCP server
	req, err := http.NewRequest(MethodGET, targetURL.String(), nil)
	if err != nil {
		log.Errorf("Failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Set SSE related request headers
	req.Header.Set(HeaderAccept, ContentTypeSSE)
	req.Header.Set(HeaderCacheControl, "no-cache")

	// Copy other request headers (except headers used internally by gateway)
	g.copyHeadersExcept(c.Request.Header, req.Header, constants.HeaderMCPServerURL)

	// Send to MCP server
	client := &http.Client{Transport: g.transport}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Failed to connect to MCP server (%s): %v", mcpServerURL, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to MCP server"})
		return
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Warnf("failed to close response body: %v", closeErr)
		}
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		log.Warnf("MCP server (%s) returned status: %d", mcpServerURL, resp.StatusCode)
		c.JSON(resp.StatusCode, gin.H{"error": "MCP server error"})
		return
	}

	// Copy response headers
	for name, values := range resp.Header {
		if name != HeaderContentType {
			for _, value := range values {
				c.Header(name, value)
			}
		}
	}

	// Set as streaming response
	c.Header(HeaderContentType, ContentTypeSSE)

	// Stream forward data
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Errorf("Error reading from MCP server (%s): %v", mcpServerURL, err)
			}
			break
		}

		if n > 0 {
			_, writeErr := c.Writer.Write(buf[:n])
			if writeErr != nil {
				log.Errorf("Error writing to client: %v", writeErr)
				break
			}

			// Flush data to client
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// handleMCPHTTPWithHeader handles MCP HTTP request (server address specified via HEADER)
func (g *MCPGateway) handleMCPHTTPWithHeader(c *gin.Context) {
	// Get MCP server address from HEADER
	mcpServerURL, err := g.getMCPSeverURL(c)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			c.JSON(mcpErr.Code, gin.H{"error": mcpErr.Message})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	// Validate URL format
	serverURL, err := g.validateAndParseURL(mcpServerURL)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			c.JSON(mcpErr.Code, gin.H{"error": mcpErr.Message})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	// HTTP path will be duplicated
	targetURL := g.buildTargetURL(serverURL, "", "")

	// Create reverse proxy with shared transport
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Header.Del(constants.HeaderMCPServerURL)
		},
		Transport: g.transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Errorf("Proxy error for server %s: %v", mcpServerURL, err)
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "Failed to forward request to MCP server",
				"details": err.Error(),
				"server":  mcpServerURL,
			})
		},
	}

	// Execute reverse proxy
	proxy.ServeHTTP(c.Writer, c.Request)
}

// arcaTargetURL resolves the target URL for an arca request. The caller's
// path is preserved after the {arca gateway prefix}/{sandbox_id} base, which
// matches what arca-sandbox SDK speaks natively.
//
// Supported engines: arca.
func (g *MCPGateway) arcaTargetURL(c *gin.Context) (*url.URL, string, error) {
	sandboxID := c.GetHeader(constants.HeaderEnvInstanceID)
	if sandboxID == "" {
		return nil, "", &MCPError{
			Code:    http.StatusBadRequest,
			Message: constants.HeaderEnvInstanceID + " header is required for arca engine",
		}
	}
	if g.config.ArcaBaseURL == "" {
		return nil, "", &MCPError{
			Code:    http.StatusInternalServerError,
			Message: "arca base URL is not configured on api-service",
		}
	}
	base, err := url.Parse(g.config.ArcaBaseURL)
	if err != nil {
		return nil, "", &MCPError{
			Code:    http.StatusInternalServerError,
			Message: "invalid arca base URL",
			Details: err.Error(),
		}
	}
	tail := strings.TrimPrefix(c.Request.URL.Path, "/")
	base.Path = fmt.Sprintf("%s/%s", arcaGatewaySandboxPrefix, sandboxID)
	if tail != "" {
		base.Path = base.Path + "/" + tail
	}
	base.RawQuery = c.Request.URL.RawQuery
	return base, sandboxID, nil
}

// handleArcaHTTP reverse-proxies non-SSE MCP traffic to the arca gateway.
// Target is derived from startup config and X-Instance-ID, not any SDK-provided URL.
//
// Supported engines: arca.
func (g *MCPGateway) handleArcaHTTP(c *gin.Context) {
	targetURL, sandboxID, err := g.arcaTargetURL(c)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			c.JSON(mcpErr.Code, gin.H{"error": mcpErr.Message})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetURL.Path
			req.URL.RawQuery = targetURL.RawQuery
			req.Host = targetURL.Host
			req.Header.Del(constants.HeaderMCPServerURL)
			req.Header.Set(arcaHeaderAPIKey, g.config.ArcaAPIKey)
			req.Header.Set(arcaHeaderSandboxID, sandboxID)
		},
		Transport: g.transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Errorf("arca proxy error for sandbox %s: %v", sandboxID, err)
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "Failed to forward request to arca gateway",
				"details": err.Error(),
				"sandbox": sandboxID,
			})
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

// handleArcaSSE streams SSE responses from the arca gateway back to the caller.
//
// Supported engines: arca.
func (g *MCPGateway) handleArcaSSE(c *gin.Context) {
	targetURL, sandboxID, err := g.arcaTargetURL(c)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			c.JSON(mcpErr.Code, gin.H{"error": mcpErr.Message})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Header(HeaderContentType, ContentTypeSSE)
	c.Header(HeaderCacheControl, "no-cache")
	c.Header(HeaderConnection, "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	req, err := http.NewRequest(MethodGET, targetURL.String(), nil)
	if err != nil {
		log.Errorf("arca SSE: failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}
	req.Header.Set(HeaderAccept, ContentTypeSSE)
	req.Header.Set(HeaderCacheControl, "no-cache")
	g.copyHeadersExcept(c.Request.Header, req.Header,
		constants.HeaderMCPServerURL,
		arcaHeaderAPIKey,
		arcaHeaderSandboxID,
	)
	req.Header.Set(arcaHeaderAPIKey, g.config.ArcaAPIKey)
	req.Header.Set(arcaHeaderSandboxID, sandboxID)

	client := &http.Client{Transport: g.transport}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("arca SSE: connect failed sandbox=%s: %v", sandboxID, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to arca gateway"})
		return
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Warnf("failed to close arca SSE response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Warnf("arca SSE: upstream status %d sandbox=%s", resp.StatusCode, sandboxID)
		c.JSON(resp.StatusCode, gin.H{"error": "arca gateway error"})
		return
	}

	for name, values := range resp.Header {
		if name != HeaderContentType {
			for _, value := range values {
				c.Header(name, value)
			}
		}
	}
	c.Header(HeaderContentType, ContentTypeSSE)

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Errorf("arca SSE read error sandbox=%s: %v", sandboxID, err)
			}
			break
		}
		if n > 0 {
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				log.Errorf("arca SSE write error: %v", writeErr)
				break
			}
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// GetRouter gets router instance
func (g *MCPGateway) GetRouter() *gin.RouterGroup {
	return g.router
}

// MCPError custom MCP error type
type MCPError struct {
	Code    int
	Message string
	Details string
}

func (e *MCPError) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}
