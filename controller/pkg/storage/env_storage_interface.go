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

package storage

import (
	"context"

	"controller/pkg/model"
)

// API interface for getting Env information from meta-service
type EnvStorage interface {
	Get(ctx context.Context, name string) (*model.Env, error)

	List(ctx context.Context, labels map[string]string) (*model.EnvList, error)

	Create(ctx context.Context, key string, env model.Env) error

	Update(ctx context.Context, key string, env model.Env) error

	Delete(ctx context.Context, key string) error
}
