// service/redis.go
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
	"api-service/models"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/go-redis/redis/v8"
)

// RedisClient wraps Redis client (same name as original var)
var RedisClientInstance *RedisClient // Global singleton (new struct)

// RedisClient struct definition
type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

// InitRedis initializes global RedisClient instance (preserves original function signature)
func InitRedis(addr, password string) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	log.Info("Connected to Redis")

	// Assign to global RedisClient (struct)
	RedisClientInstance = &RedisClient{
		client: client,
		ctx:    ctx,
	}
	return RedisClientInstance
}

// StoreEnvInstanceToRedis stores EnvInstance to Redis (signature unchanged)
func StoreEnvInstanceToRedis(token string, envInstance *models.EnvInstance) (bool, error) {
	return RedisClientInstance.StoreEnvInstanceToRedis(token, envInstance)
}

func (r *RedisClient) StoreEnvInstanceToRedis(token string, envInstance *models.EnvInstance) (bool, error) {
	key := token + ":" + envInstance.Env.Name + ":" + envInstance.Env.Version + ":" + envInstance.ID
	value, err := json.Marshal(envInstance)
	if err != nil {
		return false, fmt.Errorf("failed to marshal EnvInstance for Redis: %v", err)
	}

	err = r.client.Set(r.ctx, key, value, 0).Err()
	if err != nil {
		return false, fmt.Errorf("failed to store EnvInstance in Redis: %v", err)
	}
	return true, nil
}

// RemoveEnvInstanceFromRedis removes EnvInstance from Redis (signature unchanged)
func RemoveEnvInstanceFromRedis(token string, envInstanceId string) (bool, error) {
	return RedisClientInstance.RemoveEnvInstanceFromRedis(token, envInstanceId)
}

// RemoveEnvInstanceFromRedis removes EnvInstance from Redis (signature unchanged)
func (r *RedisClient) RemoveEnvInstanceFromRedis(token string, envInstanceId string) (bool, error) {
	err := r.deleteKeysByScan(token, envInstanceId)
	if err != nil {
		return false, fmt.Errorf("failed to delete EnvInstance from Redis: %v", err)
	}
	return true, nil
}

// GetCurrentInstanceCount gets current instance count (signature unchanged)
func GetCurrentInstanceCount(token string) (int, error) {
	return RedisClientInstance.GetCurrentInstanceCount(token)
}

func (r *RedisClient) GetCurrentInstanceCount(token string) (int, error) {
	instances, err := r.ListEnvInstancesFromRedis(token, nil)
	if err != nil {
		return 0, err
	}
	return len(instances), nil
}

// ListEnvInstancesFromRedis lists instances by token and conditions (signature unchanged)
func ListEnvInstancesFromRedis(token string, envInstance *models.EnvInstance) ([]models.EnvInstance, error) {
	return RedisClientInstance.ListEnvInstancesFromRedis(token, envInstance)
}

func (r *RedisClient) ListEnvInstancesFromRedis(token string, envInstance *models.EnvInstance) ([]models.EnvInstance, error) {
	var parts = make([]string, 0)
	if token != "" {
		parts = append(parts, token)
	} else {
		parts = append(parts, "*")
	}
	if envInstance != nil && envInstance.Env.Name != "" {
		parts = append(parts, envInstance.Env.Name)
	} else {
		parts = append(parts, "*")
	}
	if envInstance != nil && envInstance.Env.Version != "" {
		parts = append(parts, envInstance.Env.Version)
	} else {
		parts = append(parts, "*")
	}
	if envInstance != nil && envInstance.ID != "" {
		parts = append(parts, envInstance.ID)
	} else {
		parts = append(parts, "*")
	}
	pattern := strings.Join(parts, ":")

	var cursor uint64
	var keys []string

	for {
		scannedKeys, cursor, err := r.client.Scan(r.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan Redis keys with pattern %s: %v", pattern, err)
		}
		keys = append(keys, scannedKeys...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return []models.EnvInstance{}, nil
	}

	values, err := r.client.MGet(r.ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to MGet values from Redis: %v", err)
	}

	var instances []models.EnvInstance
	for i, key := range keys {
		val := values[i]
		if val == nil {
			log.Warnf("Key %s has nil value, skipping", key)
			continue
		}
		valueStr, ok := val.(string)
		if !ok {
			log.Warnf("Key %s value is not string type, skipping", key)
			continue
		}
		var instance models.EnvInstance
		if err := json.Unmarshal([]byte(valueStr), &instance); err != nil {
			log.Errorf("Failed to unmarshal value for key %s: %v", key, err)
			continue
		}
		instances = append(instances, instance)
	}

	log.Debugf("Found %d EnvInstance(s) matching pattern: %s", len(instances), pattern)
	return instances, nil
}

func (r *RedisClient) deleteKeysByScan(prefix, suffix string) error {
	pattern := prefix + "*" + suffix

	var cursor uint64
	var deletedCount int64

	for {
		keys, cursor, err := r.client.Scan(r.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if _, err := r.client.Del(r.ctx, keys...).Result(); err != nil {
				log.Errorf("Failed to delete some keys: %v", err)
			} else {
				deletedCount += int64(len(keys))
				log.Debugf("Deleted keys: %v", keys)
			}
		}

		if cursor == 0 {
			break
		}
	}

	log.Debugf("Total deleted keys matching '*%s': %d", suffix, deletedCount)
	return nil
}
