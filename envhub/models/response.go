// models/models.go
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

package models

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Unified response structure (added Message field)
type Response struct {
	Success bool        `json:"success"`
	Code    int         `json:"code"`
	Message string      `json:"message"` // New: stores prompt or error information
	Data    interface{} `json:"data"`    // Stores business data
}

// NewSuccessResponse creates a successful response
// Compatible with original semantics: Data = data, Message = ""
func NewSuccessResponse(data interface{}) *Response {
	return &Response{
		Success: true,
		Code:    0,
		Message: "",
		Data:    data,
	}
}

// NewSuccessResponseWithCode creates response with custom success code
func NewSuccessResponseWithCode(code int, data interface{}) *Response {
	return &Response{
		Success: true,
		Code:    code,
		Message: "",
		Data:    data,
	}
}

// NewErrorResponse creates error response
// Originally only code was passed, now a message is needed, but we can't get it → use status code default text
func NewErrorResponse(code int) *Response {
	return &Response{
		Success: false,
		Code:    code,
		Message: http.StatusText(code),
		Data:    nil,
	}
}

// NewErrorResponseWithData creates error response with error data
// Original intention was to put data in Data field, but now we want to promote string messages to Message
// Here we do compatibility check: if data is string, use as message; otherwise still as data
func NewErrorResponseWithData(code int, data interface{}) *Response {
	resp := &Response{
		Success: false,
		Code:    code,
		Message: http.StatusText(code), // Default message
		Data:    nil,                   // Default data
	}

	// If data is string type, treat as error message and put in Message field
	if msg, ok := data.(string); ok {
		resp.Message = msg
		resp.Data = nil
	} else {
		// Otherwise, treat as additional data and put in Data (message uses status code text)
		resp.Data = data
	}

	return resp
}

// NewEmptySuccessResponse creates empty success response
func NewEmptySuccessResponse() *Response {
	return &Response{
		Success: true,
		Code:    0,
		Message: "",
		Data:    nil,
	}
}

// NewListResponse creates list response
func NewListResponse(items interface{}, total int) *Response {
	return &Response{
		Success: true,
		Code:    0,
		Message: "",
		Data: map[string]interface{}{
			"items": items,
			"total": total,
		},
	}
}

// JSONSuccess convenience method for success response
func JSONSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, NewSuccessResponse(data))
}

// JSONSuccessWithStatus success response with status code
func JSONSuccessWithStatus(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(http.StatusOK, NewSuccessResponseWithCode(statusCode, data))
}

// JSONError convenience method for error response
// Original: returns { data: "Bad Request" } → now changed to { message: "Bad Request", data: nil }
func JSONError(c *gin.Context, statusCode int) {
	c.JSON(statusCode, NewErrorResponse(statusCode))
}

// JSONErrorWithMessage error response with custom error message
// Key function: originally data = message, now changed to message = message
func JSONErrorWithMessage(c *gin.Context, statusCode int, message string) {
	// Construct response: message field carries message, data is nil
	response := &Response{
		Success: false,
		Code:    statusCode,
		Message: message,
		Data:    nil,
	}
	c.JSON(statusCode, response)
}

// JSONErrorWithData error response with detailed error data
// data is no longer message, but additional object (e.g., debug information)
func JSONErrorWithData(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, NewErrorResponseWithData(statusCode, data))
}

// JSONList convenience method for list response
func JSONList(c *gin.Context, items interface{}, total int) {
	c.JSON(http.StatusOK, NewListResponse(items, total))
}
