package controller

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"api-service/constants"
	"api-service/service"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	arcaProxyDefaultPort       = 8081
	arcaProxyExpirationMinutes = 60
)

func (g *MCPGateway) handleArcaProxy(c *gin.Context) {
	sandboxID := strings.TrimSpace(c.GetHeader(constants.HeaderEnvInstanceID))
	if sandboxID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.HeaderEnvInstanceID + " header is required"})
		return
	}
	if g.config.ScheduleAddr == "" || g.config.ArcaAPIKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "arca proxy is not configured"})
		return
	}

	port := arcaProxyPortFromHeader(c.GetHeader(constants.HeaderMCPServerURL))
	presignURL, err := service.NewArcaClient(g.config.ScheduleAddr, g.config.ArcaAPIKey).
		PresignURL(sandboxID, port, arcaProxyExpirationMinutes)
	if err != nil {
		log.Errorf("Failed to create Arca presigned URL for sandbox %s: %v", sandboxID, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "Failed to create Arca presigned URL",
			"details": err.Error(),
		})
		return
	}

	targetURL, err := url.Parse(presignURL)
	if err != nil || targetURL.Scheme == "" || targetURL.Host == "" {
		if err != nil {
			log.Errorf("Invalid Arca presigned URL for sandbox %s: %v", sandboxID, err)
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "Invalid Arca presigned URL"})
		return
	}

	proxy := g.newArcaReverseProxy(c, targetURL, sandboxID)
	proxy.ServeHTTP(c.Writer, c.Request)
}

func arcaProxyPortFromHeader(rawURL string) int {
	if rawURL == "" {
		return arcaProxyDefaultPort
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Port() == "" {
		return arcaProxyDefaultPort
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil || port <= 0 {
		return arcaProxyDefaultPort
	}
	return port
}

func (g *MCPGateway) newArcaReverseProxy(c *gin.Context, targetURL *url.URL, sandboxID string) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = joinArcaProxyPath(targetURL.Path, req.URL.Path)
			req.URL.RawPath = ""
			req.URL.RawQuery = joinArcaProxyQuery(targetURL.RawQuery, req.URL.RawQuery)
			req.Host = targetURL.Host
			req.Header.Del(constants.HeaderEnvInstanceID)
			req.Header.Del(constants.HeaderMCPServerURL)
		},
		Transport: g.transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Errorf("Arca proxy error for sandbox %s: %v", sandboxID, err)
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "Failed to forward request to Arca sandbox",
				"details": err.Error(),
			})
		},
	}
}

func joinArcaProxyPath(basePath, requestPath string) string {
	if basePath == "" {
		basePath = "/"
	}
	if requestPath == "" || requestPath == "/" {
		return basePath
	}
	baseSlash := strings.HasSuffix(basePath, "/")
	requestSlash := strings.HasPrefix(requestPath, "/")
	switch {
	case baseSlash && requestSlash:
		return basePath + requestPath[1:]
	case !baseSlash && !requestSlash:
		return basePath + "/" + requestPath
	default:
		return basePath + requestPath
	}
}

func joinArcaProxyQuery(baseQuery, requestQuery string) string {
	if baseQuery == "" {
		return requestQuery
	}
	if requestQuery == "" {
		return baseQuery
	}
	return baseQuery + "&" + requestQuery
}
