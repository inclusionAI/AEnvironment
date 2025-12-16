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
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// Token represents an authentication token
type Token struct {
	ID               string `json:"id"`               // Unique identifier
	Token            string `json:"token"`            // Generated UUID token
	User             string `json:"user"`             // Associated user
	MaxInstanceCount int    `json:"maxInstanceCount"` // Maximum instance count limit
}

// GenerateToken generates a new Token
// Parameters:
//   - user: User identifier
//   - maxInstanceCount: Maximum instance count
//
// Returns:
//   - *Token: Generated Token object
func GenerateToken(user string, maxInstanceCount int) *Token {
	return &Token{
		ID:               uuid.New().String(), // Use UUID as ID
		Token:            uuid.New().String(), // Generate unique token
		User:             user,
		MaxInstanceCount: maxInstanceCount,
	}
}

// Optional: Generate token with prefix (e.g., "tok_")
func GenerateTokenWithPrefix(user string, maxInstanceCount int) *Token {
	return &Token{
		ID:               "tok_" + uuid.New().String()[:8], // Custom ID prefix + truncate
		Token:            uuid.New().String(),
		User:             user,
		MaxInstanceCount: maxInstanceCount,
	}
}

// Optional: Randomly generate MaxInstanceCount (for testing)
func GenerateTokenRandomLimit(user string) *Token {
	source := rand.NewSource(time.Now().UnixNano())
	limit := rand.New(source).Intn(10) + 1 // Random number between 1~10
	return GenerateToken(user, limit)
}
