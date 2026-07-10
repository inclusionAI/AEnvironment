package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type arcaEnvelope struct {
	Success bool            `json:"success"`
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *ArcaClient) doJSON(method, path string, body interface{}, extraHeaders map[string]string, out interface{}) error {
	var requestBody []byte
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("arca: marshal request: %w", err)
		}
		requestBody = data
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("arca: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(arcaAPIKeyHeader, c.apiKey)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	start := time.Now()
	logArcaRequest(method, path, url, extraHeaders, requestBody)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"method":     method,
			"path":       path,
			"url":        url,
			"latency_ms": time.Since(start).Milliseconds(),
		}).WithError(err).Error("Arca API request failed")
		return fmt.Errorf("arca: %s %s: %w", method, path, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Warnf("arca: close response body: %v", cerr)
		}
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"method":      method,
			"path":        path,
			"url":         url,
			"status_code": resp.StatusCode,
			"latency_ms":  time.Since(start).Milliseconds(),
		}).WithError(err).Error("Arca API read response failed")
		return fmt.Errorf("arca: read response: %w", err)
	}

	logArcaResponse(method, path, url, resp.StatusCode, time.Since(start), raw)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("arca: %s %s returned %d: %s", method, path, resp.StatusCode, truncateBody(raw))
	}

	var envelope arcaEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("arca: decode envelope: %w; body=%s", err, truncateBody(raw))
	}
	if !envelope.Success {
		return fmt.Errorf("arca: %s %s failed (code %s): %s", method, path, strings.Trim(string(envelope.Code), `"`), envelope.Message)
	}
	if out != nil && len(envelope.Data) > 0 && !bytes.Equal(envelope.Data, []byte("null")) {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("arca: decode data: %w; body=%s", err, truncateBody(envelope.Data))
		}
	}
	return nil
}

func logArcaRequest(method, path, url string, headers map[string]string, body []byte) {
	fields := log.Fields{
		"method":  method,
		"path":    path,
		"url":     url,
		"headers": sanitizeArcaLogHeaders(headers),
	}
	if len(body) > 0 {
		fields["request_body"] = sanitizeArcaLogBytes(body)
	}
	log.WithFields(fields).Info("Arca API request")
}

func logArcaResponse(method, path, url string, statusCode int, latency time.Duration, body []byte) {
	fields := log.Fields{
		"method":        method,
		"path":          path,
		"url":           url,
		"status_code":   statusCode,
		"latency_ms":    latency.Milliseconds(),
		"response_body": sanitizeArcaLogBytes(body),
	}
	log.WithFields(fields).Info("Arca API response")
}

func sanitizeArcaLogHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if isSensitiveArcaLogKey(k) {
			out[k] = "<redacted>"
			continue
		}
		out[k] = v
	}
	return out
}

func sanitizeArcaLogBytes(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return truncateBody(raw)
	}
	data, err := marshalArcaLogJSON(sanitizeArcaLogValue(value))
	if err != nil {
		return truncateBody(raw)
	}
	return truncateBody(data)
}

func marshalArcaLogJSON(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

func sanitizeArcaLogValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, item := range v {
			if isSensitiveArcaLogKey(k) {
				out[k] = "<redacted>"
				continue
			}
			out[k] = sanitizeArcaLogValue(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = sanitizeArcaLogValue(item)
		}
		return out
	default:
		return value
	}
}

func isSensitiveArcaLogKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "api_key")
}
