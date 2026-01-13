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
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
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
)

// MCPGateway MCP gateway struct
type MCPGateway struct {
	router *gin.RouterGroup
}

// NewMCPGateway creates a new MCP gateway instance
func NewMCPGateway(router *gin.RouterGroup) *MCPGateway {
	gateway := &MCPGateway{
		router: router,
	}

	gateway.setupRoutes()
	return gateway
}

// setupRoutes sets up routes
func (g *MCPGateway) setupRoutes() {
	g.router.Any("/*path", g.innerRouter)
}

func (g *MCPGateway) innerRouter(c *gin.Context) {
	proxyURL, _ := g.getMCPSeverURL(c)
	path := c.Param("path")
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
		log.Printf("Failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Set SSE related request headers
	req.Header.Set(HeaderAccept, ContentTypeSSE)
	req.Header.Set(HeaderCacheControl, "no-cache")

	// Copy other request headers (except headers used internally by gateway)
	g.copyHeadersExcept(c.Request.Header, req.Header, constants.HeaderMCPServerURL)

	// Send to MCP server
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to connect to MCP server (%s): %v", mcpServerURL, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to MCP server"})
		return
	}
	defer func() {
		resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("MCP server (%s) returned status: %d", mcpServerURL, resp.StatusCode)
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
				log.Printf("Error reading from MCP server (%s): %v", mcpServerURL, err)
			}
			break
		}

		if n > 0 {
			_, writeErr := c.Writer.Write(buf[:n])
			if writeErr != nil {
				log.Printf("Error writing to client: %v", writeErr)
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

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(&targetURL)

	// Error handling
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error for server %s: %v", mcpServerURL, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "Failed to forward request to MCP server",
			"details": err.Error(),
			"server":  mcpServerURL,
		})
	}

	// Create Director to modify request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Execute original Director first
		originalDirector(req)

		// Remove headers used internally by gateway
		req.Header.Del(constants.HeaderMCPServerURL)
	}

	// Execute reverse proxy
	proxy.ServeHTTP(c.Writer, c.Request)
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
