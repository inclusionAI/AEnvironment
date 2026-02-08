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

package aenvhub_http_server

import "time"

// HttpResponseData represents container/pod response data
type HttpResponseData struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	IP     string `json:"ip"`
	TTL    string `json:"ttl"`
	Owner  string `json:"owner,omitempty"` // Optional, may be empty
}

// HttpResponse represents standard HTTP response
type HttpResponse struct {
	Success      bool             `json:"success"`
	Code         int              `json:"code"`
	ResponseData HttpResponseData `json:"data"`
}

// HttpDeleteResponse represents delete response
type HttpDeleteResponse struct {
	Success      bool `json:"success"`
	Code         int  `json:"code"`
	ResponseData bool `json:"data"`
}

// HttpListResponseData represents container/pod list item
type HttpListResponseData struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	TTL       string    `json:"ttl"`
	CreatedAt time.Time `json:"created_at"`
	EnvName   string    `json:"envname"`
	Version   string    `json:"version"`
	IP        string    `json:"ip"`
	Owner     string    `json:"owner,omitempty"` // Optional, may be empty
}

// HttpListResponse represents list response
type HttpListResponse struct {
	Success          bool                   `json:"success"`
	Code             int                    `json:"code"`
	ListResponseData []HttpListResponseData `json:"data"`
}
