package faas_model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HTTPClient a http client in RESTful style
// the http client can be used like:
// c := NewHTTPClient(baseURL)
// c.Get("/some/url").Headers().Do().Into(&MyStruct{})
// c.Post("/some/url").Body("{}").Do().Into(&MyStruct{})
type HTTPClient struct {
	*http.Client
	BaseURL string
}

// NewHTTPClient creates a new http client
// baseURL is the base url for all requests
// returns an instance of HTTPClient
// Example:
//
//	c := NewHTTPClient("https://example.com")
//	resp, err := c.Get("/users").QueryParam("id", "123").Do()
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		Client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSHandshakeTimeout: 5 * time.Second,
			},
		},
		BaseURL: baseURL,
	}
}

// Get creates a GET request
func (c *HTTPClient) Get(path string) *HTTPReq {
	return c.do(http.MethodGet, path)
}

// Delete creates a DELETE request
func (c *HTTPClient) Delete(path string) *HTTPReq {
	return c.do(http.MethodDelete, path)
}

// Post creates a POST request
func (c *HTTPClient) Post(path string) *HTTPReq {
	return c.do(http.MethodPost, path)
}

// Put creates a PUT request
func (c *HTTPClient) Put(path string) *HTTPReq {
	return c.do(http.MethodPut, path)
}

func (c *HTTPClient) do(method string, path string) *HTTPReq {
	return NewHTTPReqWithMethodPath(c.Client, c.BaseURL, method, path)
}

// HTTPReq represents a http request
type HTTPReq struct {
	// client is the http client to send the real http request
	client *http.Client

	// timeout is the timeout duration for this request
	// default value is 10 seconds
	timeout time.Duration

	// internal representations of the request
	headers map[string]string
	baseURL string
	method  string
	path    string
	body    []byte
	q       url.Values

	resp *http.Response

	errors []error
}

// NewHTTPReq creates a new http req
func NewHTTPReq(c *http.Client) *HTTPReq {
	return &HTTPReq{
		client: c,

		headers: map[string]string{},

		errors: make([]error, 0),
	}
}

func NewHTTPReqWithMethodPath(c *http.Client, baseURL, method, path string) *HTTPReq {
	return &HTTPReq{
		client:  c,
		baseURL: baseURL,
		method:  method,
		path:    path,
		headers: map[string]string{},
		errors:  make([]error, 0),
	}
}

// recordError records errors
// if there are any errors, they will be recorded here
func (r *HTTPReq) recordError(err error) {
	if err != nil {
		if r.errors == nil {
			r.errors = make([]error, 0)
		}
		r.errors = append(r.errors, err)
	}
}

// Do will send the request
// returns the same instance of HTTPReq
// If http response is wanted, caller should call Response() method after
// Example usage:
// r := c.Get("/user").QueryParam("name", "john").Do()
// resp, err := r.Response()
func (r *HTTPReq) Do() *HTTPReq {
	fullPath := r.baseURL + r.path
	if r.q != nil {
		fullPath = fullPath + "?" + r.q.Encode()
	}

	req, err := http.NewRequest(r.method, fullPath, bytes.NewBuffer(r.body))
	if err != nil {
		r.recordError(fmt.Errorf("failed to create http request: %v", err))
		return r
	}

	if req.Header.Get("Content-Type") == "" {
		req.Header.Add("Content-Type", "application/json")
	}
	for k, v := range r.headers {
		req.Header.Add(k, v)
	}

	if r.timeout != 0 {
		r.client.Timeout = r.timeout
	}

	resp, err := r.client.Do(req)
	if err != nil {
		r.recordError(fmt.Errorf("failed to send http request: %v", err))
		return r
	}

	r.resp = resp
	return r
}

func (r *HTTPReq) Timeout(t time.Duration) *HTTPReq {
	r.timeout = t
	return r
}

// Headers sets headers on this request
func (r *HTTPReq) Headers(headers map[string]string) *HTTPReq {
	for k, v := range headers {
		r.headers[k] = v
	}

	return r
}

// Body will set the body of this request
// the body should be a pointer to an empty struct
// it will be marshaled and sent as the body
func (r *HTTPReq) Body(in interface{}) *HTTPReq {
	data, err := json.Marshal(in)
	if err != nil {
		r.recordError(fmt.Errorf("failed to marshal body: %v", err))
		return r
	}

	r.body = data
	return r
}

func (r *HTTPReq) BodyData(data []byte) *HTTPReq {
	r.body = data
	return r
}

func (r *HTTPReq) Query(key string, value string) *HTTPReq {
	if r.q == nil {
		r.q = url.Values{}
	}

	r.q.Set(key, value)
	return r
}

// Into will unmarshal the response into the given object
// the obj should be a pointer to an empty struct
// if the response is not 200, the error object will be unmarshalled into e
func (r *HTTPReq) Into(obj interface{}, e ...interface{}) error {
	if len(r.errors) > 0 {
		return r.errors[0]
	}

	if r.resp == nil {
		return fmt.Errorf("response is not ready")
	}

	data, err := io.ReadAll(r.resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// If execution is successful, parse the body content into the object; otherwise return an error
	if r.resp.StatusCode >= http.StatusOK && r.resp.StatusCode < 300 {
		if err := json.Unmarshal(data, obj); err != nil {
			return fmt.Errorf("failed to unmarshal response data: %s. err: %v", data, err)
		}
		return nil
	} else {
		// try to unmarshal the error message into known struct
		if len(e) > 0 {
			if err := json.Unmarshal(data, e[0]); err != nil {
				return fmt.Errorf("http request with non-200 status code: %d, body: %s", r.resp.StatusCode, string(data))
			}
		}

		return fmt.Errorf("http request with non-200 status code: %d, body: %s", r.resp.StatusCode, string(data))
	}
}

// Response will return the http response
// if there're any errors during sending or receiving,
// the first error will be returned
func (r *HTTPReq) Response() (*http.Response, error) {
	if len(r.errors) > 0 {
		return nil, r.errors[0]
	}

	if r.resp == nil {
		return nil, fmt.Errorf("response is not ready")
	}

	return r.resp, nil
}

// Constants for HTTP headers
const (
	HttpHeaderInstanceID  = "Hcs-Faas-Instance-Id"
	LabelStatefulFunction = "Hcs-Faas-Stateful-Function"
)
