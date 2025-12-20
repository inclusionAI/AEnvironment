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
	"errors"
	"fmt"
	"sort"
	"time"

	redis "github.com/go-redis/redis/v8"

	"envhub/models"
)

// ErrEnvNotFound indicates that the specified Env was not found in Redis
var ErrEnvNotFound = errors.New("env not found")

// RedisEnvStorageOptions Redis storage configuration
type RedisEnvStorageOptions struct {
	// Direct connection mode: specify Addr to connect directly to Redis master
	Addr      string
	// Sentinel mode: specify SentinelAddrs and MasterName to use Sentinel
	SentinelAddrs []string // Sentinel addresses (e.g., ["sentinel1:26379", "sentinel2:26379"])
	MasterName    string   // Master name in Sentinel (default: "mymaster")
	// Common options
	Username  string
	Password  string
	DB        int
	KeyPrefix string
}

// RedisEnvStorage implements EnvStorage interface using Redis
type RedisEnvStorage struct {
	client    *redis.Client
	keyPrefix string
	indexKey  string
}

var _ EnvStorage = (*RedisEnvStorage)(nil)

// NewRedisEnvStorage creates RedisEnvStorage
// Supports both direct connection and Sentinel mode
func NewRedisEnvStorage(opts RedisEnvStorageOptions) (*RedisEnvStorage, error) {
	if opts.KeyPrefix == "" {
		opts.KeyPrefix = "env"
	}

	var client *redis.Client

	// Check if using Sentinel mode
	if len(opts.SentinelAddrs) > 0 {
		// Sentinel mode
		if opts.MasterName == "" {
			opts.MasterName = "mymaster" // Default master name for Bitnami Redis Chart
		}

		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    opts.MasterName,
			SentinelAddrs: opts.SentinelAddrs,
			Username:      opts.Username,
			Password:      opts.Password,
			DB:            opts.DB,
		})
	} else {
		// Direct connection mode
		if opts.Addr == "" {
			return nil, fmt.Errorf("redis addr must not be empty when not using Sentinel mode")
		}
		client = redis.NewClient(&redis.Options{
			Addr:     opts.Addr,
			Username: opts.Username,
			Password: opts.Password,
			DB:       opts.DB,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisEnvStorage{
		client:    client,
		keyPrefix: opts.KeyPrefix,
		indexKey:  fmt.Sprintf("%s:index", opts.KeyPrefix),
	}, nil
}

type redisEnvRecord struct {
	Env              *models.Env       `json:"env"`
	Labels           map[string]string `json:"labels"`
	ResourceVersion  int64             `json:"resourceVersion"`
	LastUpdatedEpoch int64             `json:"lastUpdatedEpoch"`
}

func (s *RedisEnvStorage) dataKey(key string) string {
	return fmt.Sprintf("%s:data:%s", s.keyPrefix, key)
}

// Get gets Env object by key
func (s *RedisEnvStorage) Get(ctx context.Context, key string) (*models.Env, int64, error) {
	record, err := s.loadRecord(ctx, key)
	if err != nil {
		return nil, 0, err
	}
	return record.Env, record.ResourceVersion, nil
}

// Create creates Env object
func (s *RedisEnvStorage) Create(ctx context.Context, key string, env *models.Env, labels map[string]string) error {
	record := &redisEnvRecord{
		Env:              env,
		Labels:           copyLabels(labels),
		ResourceVersion:  1,
		LastUpdatedEpoch: time.Now().Unix(),
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal env record: %w", err)
	}

	redisKey := s.dataKey(key)
	ok, err := s.client.SetNX(ctx, redisKey, payload, 0).Result()
	if err != nil {
		return fmt.Errorf("failed to create env in redis: %w", err)
	}
	if !ok {
		return fmt.Errorf("env %s already exists", key)
	}

	if err := s.client.SAdd(ctx, s.indexKey, key).Err(); err != nil {
		return fmt.Errorf("failed to update redis index: %w", err)
	}
	return nil
}

// Update updates Env object
func (s *RedisEnvStorage) Update(ctx context.Context, key string, env *models.Env, resourceVersion int64, labels map[string]string) error {
	redisKey := s.dataKey(key)
	return s.client.Watch(ctx, func(tx *redis.Tx) error {
		payload, err := tx.Get(ctx, redisKey).Bytes()
		if errors.Is(err, redis.Nil) {
			return fmt.Errorf("%w: %s", ErrEnvNotFound, key)
		}
		if err != nil {
			return fmt.Errorf("failed to read env %s: %w", key, err)
		}

		var record redisEnvRecord
		if err := json.Unmarshal(payload, &record); err != nil {
			return fmt.Errorf("failed to unmarshal env %s: %w", key, err)
		}

		if record.ResourceVersion != resourceVersion {
			return fmt.Errorf("resource version mismatch for %s: expect %d got %d", key, record.ResourceVersion, resourceVersion)
		}

		record.Env = env
		if labels != nil {
			record.Labels = copyLabels(labels)
		}
		record.ResourceVersion++
		record.LastUpdatedEpoch = time.Now().Unix()

		updatedPayload, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal updated env %s: %w", key, err)
		}

		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.Set(ctx, redisKey, updatedPayload, 0)
			p.SAdd(ctx, s.indexKey, key)
			return nil
		})
		return err
	}, redisKey)
}

// Delete deletes Env object
func (s *RedisEnvStorage) Delete(ctx context.Context, key string) error {
	redisKey := s.dataKey(key)
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, redisKey)
	pipe.SRem(ctx, s.indexKey, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete env %s: %w", key, err)
	}
	return nil
}

// List lists all Env keys, optional label filtering
func (s *RedisEnvStorage) List(ctx context.Context, labels map[string]string) ([]string, error) {
	keys, err := s.client.SMembers(ctx, s.indexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list env keys: %w", err)
	}
	if len(labels) == 0 {
		sort.Strings(keys)
		return keys, nil
	}

	var filtered []string
	for _, key := range keys {
		record, err := s.loadRecord(ctx, key)
		if err != nil {
			if errors.Is(err, ErrEnvNotFound) || errors.Is(err, redis.Nil) {
				// Clean dirty data, don't affect request
				_ = s.client.SRem(ctx, s.indexKey, key).Err()
				continue
			}
			return nil, err
		}
		if labelsMatch(labels, record.Labels) {
			filtered = append(filtered, key)
		}
	}
	sort.Strings(filtered)
	return filtered, nil
}

// Watch watches for Env changes (Redis implementation does not support this yet)
func (s *RedisEnvStorage) Watch(ctx context.Context, rv int64, key string, labels map[string]string) (WatchClient, error) {
	return nil, fmt.Errorf("redis env storage does not support watch")
}

func (s *RedisEnvStorage) loadRecord(ctx context.Context, key string) (*redisEnvRecord, error) {
	payload, err := s.client.Get(ctx, s.dataKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("%w: %s", ErrEnvNotFound, key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load env %s: %w", key, err)
	}
	var record redisEnvRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, fmt.Errorf("failed to decode env %s: %w", key, err)
	}
	return &record, nil
}

func copyLabels(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func labelsMatch(expected, actual map[string]string) bool {
	if len(expected) == 0 {
		return true
	}
	if len(actual) == 0 {
		return false
	}
	for k, v := range expected {
		if actual[k] != v {
			return false
		}
	}
	return true
}
