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
	"fmt"

	"envhub/models" // Ensure path is correct
)

// TokenStorage provides storage interface for Token objects
// TODO: TokenStorage needs to be migrated to use Redis instead of MetaServiceClient
// This is currently disabled in main.go until Redis implementation is ready
type TokenStorage struct {
	// client *client.MetaServiceClient // Removed: MetaServiceClient dependency
}

// NewTokenStorage creates TokenStorage instance
// TODO: Update to use Redis client instead of MetaServiceClient
func NewTokenStorage() *TokenStorage {
	return &TokenStorage{}
}

// Get gets Token object by key
// TODO: Implement using Redis
func (s *TokenStorage) Get(ctx context.Context, key string) (*models.Token, int64, error) {
	return nil, 0, fmt.Errorf("TokenStorage.Get not implemented: needs Redis migration")
}

// Create creates Token object
// TODO: Implement using Redis
func (s *TokenStorage) Create(ctx context.Context, key string, token *models.Token, labels map[string]string) error {
	return fmt.Errorf("TokenStorage.Create not implemented: needs Redis migration")
}

// Update updates Token object
// TODO: Implement using Redis
func (s *TokenStorage) Update(ctx context.Context, key string, token *models.Token, resourceVersion int64, labels map[string]string) error {
	return fmt.Errorf("TokenStorage.Update not implemented: needs Redis migration")
}

// Delete deletes Token object
// TODO: Implement using Redis
func (s *TokenStorage) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("TokenStorage.Delete not implemented: needs Redis migration")
}

// List lists all Token keys (optional: filter by labels)
// TODO: Implement using Redis
func (s *TokenStorage) List(ctx context.Context, labels map[string]string) ([]string, error) {
	return nil, fmt.Errorf("TokenStorage.List not implemented: needs Redis migration")
}

// Watch watches for changes to Token
// TODO: Implement using Redis
func (s *TokenStorage) Watch(ctx context.Context, rv int64, key string, labels map[string]string) (WatchClient, error) {
	return nil, fmt.Errorf("TokenStorage.Watch not implemented: needs Redis migration")
}

func (s *TokenStorage) GetByUser(ctx context.Context, user string) ([]*models.Token, error) {
	keys, err := s.List(ctx, map[string]string{"user": user})
	if err != nil {
		return nil, err
	}
	var tokens []*models.Token
	for _, key := range keys {
		token, _, err := s.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// DeleteByUser deletes all Tokens for specified user
// Parameters:
//   - ctx: context
//   - user: user identifier (e.g., "alice")
//
// Returns:
//   - error: returns error if any deletion fails (can choose whether to continue)
func (s *TokenStorage) DeleteByUser(ctx context.Context, user string) error {
	// Use label filter: assume label user=xxx was set during creation
	labels := map[string]string{
		"user": user,
	}

	// Get all token keys for this user
	keys, err := s.List(ctx, labels)
	if err != nil {
		return fmt.Errorf("failed to list tokens for user %s: %w", user, err)
	}

	if len(keys) == 0 {
		// No Token found, consider as success
		return nil
	}

	var deleteErrors []string

	// Iterate and delete each Token
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			deleteErrors = append(deleteErrors, fmt.Sprintf("key=%s: %v", key, err))
		}
	}

	// If any deletion failed, return aggregated error
	if len(deleteErrors) > 0 {
		return fmt.Errorf("failed to delete some tokens for user %s: %s", user, joinErrors(deleteErrors, "; "))
	}

	return nil
}

// Helper function: merge error messages
func joinErrors(errors []string, sep string) string {
	if len(errors) == 0 {
		return ""
	}
	result := errors[0]
	for _, e := range errors[1:] {
		result += sep + e
	}
	return result
}
