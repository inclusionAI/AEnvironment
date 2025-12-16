// controller/token_controller.go
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
	"context"

	"envhub/models"
	"envhub/service"

	"github.com/gin-gonic/gin"
)

// TokenController handles Token related requests
type TokenController struct {
	storage *service.TokenStorage
}

// NewTokenController creates TokenController instance
func NewTokenController(storage *service.TokenStorage) *TokenController {
	return &TokenController{
		storage: storage,
	}
}

// RegisterTokenRoutes registers token related routes
func (c *TokenController) RegisterTokenRoutes(r *gin.Engine) {
	tokenGroup := r.Group("/token")

	// Apply for token
	tokenGroup.POST("/", c.ApplyToken)
	// Update limit
	tokenGroup.PUT("/", c.UpdateTokenLimit)

	// New: validate if token exists
	tokenGroup.GET("/validate/:token", c.ValidateToken)

	// Query token or user information (original GetTokenLimits renamed and kept compatible)
	tokenGroup.GET("/info/:id", c.GetTokenInfo)

	// Delete token
	tokenGroup.DELETE("/:id", c.DeleteToken)
}

// ValidateToken validates if token exists
// GET /token/validate/:token
func (c *TokenController) ValidateToken(ctx *gin.Context) {
	tokenStr := ctx.Param("token")
	if tokenStr == "" {
		models.JSONErrorWithMessage(ctx, 400, "Missing token parameter")
		return
	}

	// Directly query token (exact match)
	token, _, err := c.storage.Get(ctx, tokenStr)
	if err != nil || token == nil {
		models.JSONErrorWithMessage(ctx, 404, "Invalid token")
		return
	}

	// Return token basic information (optionally desensitized)
	models.JSONSuccess(ctx, token)
}

// GetTokenInfo queries token or user limit (compatible with querying user)
// GET /token/info/:id
func (c *TokenController) GetTokenInfo(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		models.JSONErrorWithMessage(ctx, 400, "Missing query identifier")
		return
	}

	// First try to query as token
	token, _, err := c.storage.Get(ctx, id)
	if err == nil && token != nil {
		models.JSONSuccess(ctx, token)
		return
	}

	// Then try to query as user
	tokens, err := c.storage.GetByUser(ctx, id)
	if err != nil || len(tokens) == 0 {
		models.JSONErrorWithMessage(ctx, 404, "Token or user not found")
		return
	}

	// Return first token (assume one token per user)
	models.JSONSuccess(ctx, tokens[0])
}

// ApplyToken applies for Token
// POST /token/
func (c *TokenController) ApplyToken(ctx *gin.Context) {
	var req struct {
		User             string `json:"user" binding:"required"`
		MaxInstanceCount int    `json:"maxInstanceCount"`
	}
	if req.MaxInstanceCount == 0 {
		req.MaxInstanceCount = 2
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.JSONErrorWithMessage(ctx, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// Check if user already exists
	tokens, err := c.storage.GetByUser(ctx, req.User)
	if err != nil {
		models.JSONErrorWithMessage(ctx, 500, "Internal server error: failed to query user")
		return
	}
	if len(tokens) > 0 {
		models.JSONErrorWithMessage(ctx, 400, "User already exists")
		return
	}

	// Generate new Token
	tokenObj := models.GenerateToken(req.User, req.MaxInstanceCount)
	key := tokenObj.Token

	// Add label for easy querying
	labels := map[string]string{
		"user":  req.User,
		"token": tokenObj.Token,
	}

	// Create storage
	if err := c.storage.Create(ctx, key, tokenObj, labels); err != nil {
		models.JSONErrorWithMessage(ctx, 500, "Failed to create token: "+err.Error())
		return
	}

	// Return success response
	models.JSONSuccess(ctx, tokenObj)
}

// UpdateTokenLimit updates token's maximum instance count limit
// PUT /token/
func (c *TokenController) UpdateTokenLimit(ctx *gin.Context) {
	var req struct {
		Token            string `json:"token"`
		User             string `json:"user"`
		MaxInstanceCount int    `json:"maxInstanceCount" binding:"required,gte=1"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		models.JSONErrorWithMessage(ctx, 400, "Invalid request parameters: "+err.Error())
		return
	}
	if req.Token == "" && req.User == "" {
		models.JSONErrorWithMessage(ctx, 400, "Missing token or user identifier")
		return
	}
	ctxB := context.Background()
	var token *models.Token
	var resourceVersion int64
	var err error
	var key string
	// Prefer using token to query
	if req.Token != "" {
		token, resourceVersion, err = c.storage.Get(ctxB, req.Token)
		if err == nil && token != nil {
			key = req.Token
		}
	}
	// If token not found, try to query by user
	if token == nil && req.User != "" {
		tokens, err := c.storage.GetByUser(ctxB, req.User)
		if err == nil && len(tokens) > 0 {
			token = tokens[0]
			key = token.Token
			_, resourceVersion, _ = c.storage.Get(ctxB, key)
		}
	}
	// Still not found
	if token == nil {
		models.JSONErrorWithMessage(ctx, 404, "Token does not exist")
		return
	}
	// Update MaxInstanceCount field
	token.MaxInstanceCount = req.MaxInstanceCount
	// Preserve original labels
	labels := map[string]string{
		"user":  token.User,
		"token": token.Token,
	}
	// Execute update
	if err := c.storage.Update(ctxB, key, token, resourceVersion, labels); err != nil {
		models.JSONErrorWithMessage(ctx, 500, "Update failed: "+err.Error())
		return
	}
	// Return updated object
	models.JSONSuccess(ctx, token)
}

// DeleteToken deletes token (supports deletion by token or user)
// DELETE /token/:id
func (c *TokenController) DeleteToken(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		models.JSONErrorWithMessage(ctx, 400, "Missing delete identifier")
		return
	}

	ctxB := context.Background()
	var deleteErr error

	// Try to delete as token
	deleteErr = c.storage.Delete(ctxB, id)
	if deleteErr == nil {
		models.JSONSuccess(ctx, "Deleted successfully")
		return
	}

	// Try to delete all tokens as user
	deleteErr = c.storage.DeleteByUser(ctxB, id)
	if deleteErr != nil {
		models.JSONErrorWithMessage(ctx, 500, "Delete failed: "+deleteErr.Error())
		return
	}

	models.JSONSuccess(ctx, "Deleted successfully")
}
