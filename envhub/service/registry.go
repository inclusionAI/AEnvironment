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
	"fmt"
	"sort"
	"sync"
)

// EnvStorageFactory builds an EnvStorage instance from a backend-specific options map.
//
// The options map carries CLI / env-supplied parameters (e.g. addr, prefix). Backends only
// read keys they understand; unknown keys must be ignored. This keeps the main loop free
// of backend-specific types while letting external implementations register themselves
// via init().
type EnvStorageFactory func(opts map[string]string) (EnvStorage, error)

// BlobStorageFactory builds a BlobStorage instance. Same convention as EnvStorageFactory.
type BlobStorageFactory func(opts map[string]string) (BlobStorage, error)

var (
	envStorageMu  sync.RWMutex
	envStorageReg = map[string]EnvStorageFactory{}

	blobStorageMu  sync.RWMutex
	blobStorageReg = map[string]BlobStorageFactory{}
)

// RegisterEnvStorage registers a named EnvStorage backend factory. Intended to be called
// from `init()` of either built-in implementations (redis) or external adapters.
//
// Re-registering an existing name overrides the previous factory. This is intentional: it
// lets alternative builds replace defaults without forking the code.
func RegisterEnvStorage(name string, f EnvStorageFactory) {
	if name == "" || f == nil {
		return
	}
	envStorageMu.Lock()
	defer envStorageMu.Unlock()
	envStorageReg[name] = f
}

// BuildEnvStorage looks up a registered EnvStorage backend and constructs it.
func BuildEnvStorage(name string, opts map[string]string) (EnvStorage, error) {
	envStorageMu.RLock()
	f, ok := envStorageReg[name]
	envStorageMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported env storage backend %q (registered: %v)", name, ListEnvStorageBackends())
	}
	return f(opts)
}

// ListEnvStorageBackends returns the sorted list of registered env storage backend names.
func ListEnvStorageBackends() []string {
	envStorageMu.RLock()
	defer envStorageMu.RUnlock()
	names := make([]string, 0, len(envStorageReg))
	for n := range envStorageReg {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// RegisterBlobStorage registers a named BlobStorage backend factory.
func RegisterBlobStorage(name string, f BlobStorageFactory) {
	if name == "" || f == nil {
		return
	}
	blobStorageMu.Lock()
	defer blobStorageMu.Unlock()
	blobStorageReg[name] = f
}

// BuildBlobStorage looks up a registered BlobStorage backend.
//
// Unlike EnvStorage, BlobStorage is optional: if the requested backend is not registered
// AND no opts are supplied, BuildBlobStorage returns (nil, nil) so callers can degrade
// gracefully. If a name is explicitly requested but not registered, an error is returned.
func BuildBlobStorage(name string, opts map[string]string) (BlobStorage, error) {
	if name == "" {
		return nil, nil
	}
	blobStorageMu.RLock()
	f, ok := blobStorageReg[name]
	blobStorageMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported blob storage backend %q (registered: %v)", name, ListBlobStorageBackends())
	}
	return f(opts)
}

// ListBlobStorageBackends returns the sorted list of registered blob storage backend names.
func ListBlobStorageBackends() []string {
	blobStorageMu.RLock()
	defer blobStorageMu.RUnlock()
	names := make([]string, 0, len(blobStorageReg))
	for n := range blobStorageReg {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
