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

import "os"

// ConfigProvider resolves a configuration key to its current string value.
//
// The default is EnvConfigProvider (os.Getenv). Downstream builds may install
// alternative providers via SetConfigProvider so callers do not have to import
// implementation-specific SDKs.
type ConfigProvider interface {
	Get(key string) (value string, ok bool)
}

// EnvConfigProvider reads configuration from process environment variables.
type EnvConfigProvider struct{}

func (EnvConfigProvider) Get(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	return v, ok
}

var activeConfigProvider ConfigProvider = EnvConfigProvider{}

// SetConfigProvider installs a process-wide ConfigProvider. Safe to call at program start
// before storage/blob backends are built. Passing nil resets to the env-based default.
func SetConfigProvider(p ConfigProvider) {
	if p == nil {
		activeConfigProvider = EnvConfigProvider{}
		return
	}
	activeConfigProvider = p
}

// GetConfig returns the value for key via the active ConfigProvider, falling back to def
// when the key is absent or empty.
func GetConfig(key, def string) string {
	if v, ok := activeConfigProvider.Get(key); ok && v != "" {
		return v
	}
	return def
}
