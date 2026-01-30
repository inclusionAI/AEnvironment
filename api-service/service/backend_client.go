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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	backendmodel "envhub/models"
	"io"
	"net/http"

	"api-service/models"

	lru "github.com/hashicorp/golang-lru/v2"
)

type BackendClientInterface interface {
	ValidateToken(token string) (*backendmodel.Token, error)
}

// BackendClient Backend service client
type BackendClient struct {
	baseURL    string
	httpClient *http.Client
	cache      *lru.Cache[string, cachedToken] // Cache token -> Token object (nil means invalid)
	ttl        time.Duration                   // TTL controls cache expiration (for passive invalidation)
}

// cachedToken represents a cache item with expiration time
type cachedToken struct {
	value    *backendmodel.Token
	expireAt time.Time
}

// NewBackendClient creates a Backend client
// maxCacheEntries: maximum cache entries, 0 means unlimited (recommend setting a reasonable value)
// ttl: cache expiration time, 0 means never expires (not recommended)
func NewBackendClient(backendAddr string, maxCacheEntries int, ttl time.Duration) (*BackendClient, error) {
	if maxCacheEntries <= 0 {
		maxCacheEntries = 1000 // Default 1000 entries
	}
	if ttl <= 0 {
		ttl = 1 * time.Minute // Default 1 minute
	}
	cache, err := lru.New[string, cachedToken](maxCacheEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}
	client := &BackendClient{
		baseURL: backendAddr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: cache,
		ttl:   ttl,
	}
	// Start periodic cleanup task (actively delete expired items)
	if ttl > 0 {
		go client.startCleanup(ttl, context.Background())
	}
	return client, nil
}

// startCleanup periodically cleans expired cache (simple TTL implementation)
// Current approach: doesn't clear on each cleanup, relies on external TTL check; better approach is to record timestamp
// Left empty here as a future extension point (e.g., combine time.Now() + expTime storage)
func (c *BackendClient) startCleanup(interval time.Duration, ctx context.Context) {
	ticker := time.NewTicker(interval / 2) // Scan every half-TTL
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			// Get all current keys (snapshot)
			keys := c.cache.Keys() // Note: Keys() returns ordered list, not real-time view

			var expiredKeys []string
			for _, key := range keys {
				if val, ok := c.cache.Peek(key); ok { // Peek doesn't update LRU order
					if now.After(val.expireAt) {
						expiredKeys = append(expiredKeys, key)
					}
				}
			}

			// Batch delete expired keys
			for _, key := range expiredKeys {
				c.cache.Remove(key)
			}

			if len(expiredKeys) > 0 {
				log.Infof("Cleaned up %d expired tokens from cache", len(expiredKeys))
			}
		}
	}
}

// GetEnvByVersion fetches environment
// GET /env/{name}/{version}
func (c *BackendClient) GetEnvByVersion(name, version string) (*backendmodel.Env, error) {
	url := fmt.Sprintf("%s/env/%s/%s", c.baseURL, name, version)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}
	var envResp models.ClientResponse[backendmodel.Env]
	if err := json.Unmarshal(body, &envResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	return &envResp.Data, nil
}

// ValidateToken validates if token is valid and caches its information
// Returns: (is valid, error)
func (c *BackendClient) ValidateToken(token string) (*backendmodel.Token, error) {
	if token == "" {
		return nil, nil
	}

	// 1. Try to read from cache and validate expiration
	if cached, ok := c.cache.Get(token); ok {
		if time.Now().Before(cached.expireAt) {
			// Valid: return value (may be nil, indicating invalid token is cached)
			return cached.value, nil
		} else {
			// Expired, remove
			c.cache.Remove(token)
		}
	}

	// 2. Cache miss or expired, request backend
	url := fmt.Sprintf("%s/token/validate/%s", c.baseURL, token)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	// 3. Parse response
	var tokenResp models.ClientResponse[*backendmodel.Token]
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 4. Determine validity and write to cache with TTL
	var tokenValue *backendmodel.Token
	isValid := tokenResp.Success && tokenResp.Code == 0 && tokenResp.Data != nil
	if isValid {
		tokenValue = tokenResp.Data
	} // Otherwise nil (cache penetration protection)

	// Calculate expiration time
	expireAt := time.Now().Add(c.ttl)

	// Write to cache
	c.cache.Add(token, cachedToken{
		value:    tokenValue,
		expireAt: expireAt,
	})

	return tokenValue, nil
}

func (c *BackendClient) SearchDatasource(scenario, key string) (string, error) {
	url := fmt.Sprintf("%s/data?scenario=%s&instance=%s", c.baseURL, scenario, key)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}
	var searchResp models.ClientResponse[interface{}]
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}
	target := searchResp.Data
	if target == nil {
		return "", nil
	}
	if m, ok := target.(map[string]interface{}); ok {
		if nameVal, exists := m["image"]; exists {
			if name, ok := nameVal.(string); ok {
				return name, nil
			}
		}
	}
	return "", nil
}
