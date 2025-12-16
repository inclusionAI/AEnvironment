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

	"envhub/models" // Replace with your actual package path

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
)

// WatchClient defines the interface for watching changes
// This replaces the metaservice/pkg/message.MessageServer_WatchClient dependency
type WatchClient interface {
	Recv() (interface{}, error)
	Close() error
}

// EnvStorage defines storage interface for Env objects
type EnvStorage interface {
	Get(ctx context.Context, key string) (*models.Env, int64, error)
	Create(ctx context.Context, key string, env *models.Env, labels map[string]string) error
	Update(ctx context.Context, key string, env *models.Env, resourceVersion int64, labels map[string]string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, labels map[string]string) ([]string, error)
	Watch(ctx context.Context, rv int64, key string, labels map[string]string) (WatchClient, error)
}

type OssStorage struct {
	client *oss.Client
	config *OssConfig
}

func NewOssStorage(config *OssConfig) (*OssStorage, error) {
	// If OSS is not configured, return nil without error
	if !IsOssConfigured(config) {
		return nil, nil
	}
	client, err := makeOssClient(config)
	if err != nil {
		return nil, err
	}
	return &OssStorage{
		client: client,
		config: config,
	}, nil
}

func (oss *OssStorage) PresignEnv(env string, style string) (string, error) {
	if oss == nil {
		return "", fmt.Errorf("OSS storage is not configured")
	}
	var request any
	if style == "read" {
		request = makeEnvReadRequest(oss.config, env)
	} else {
		request = makeEnvUploadRequest(oss.config, env)
	}
	envPresign, err := oss.client.Presign(context.TODO(), request)
	if err != nil {
		return "", err
	}
	//expiration := envPresign.Expiration
	return envPresign.URL, nil
}
