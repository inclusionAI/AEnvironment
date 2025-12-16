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
)

type Datasource struct {
	Image    string `json:"image"`
	Instance string `json:"instance"`
}

type DataSearch struct {
	Key      string `json:"key"`
	Scenario string `json:"scenario"`
}

type DatesetService interface {
	Load(ctx context.Context, search DataSearch) *Datasource
}

type StaticDatasetService struct {
}

type SweBenchDataService struct {
	mappings map[string]string
}

func NewSweBenchDataService() *SweBenchDataService {
	content := readConfig("/datasource/swe-verify-images.json")
	var config map[string]string
	err := json.Unmarshal([]byte(content), &config)
	if err != nil {
		config = make(map[string]string)
	}
	return &SweBenchDataService{
		mappings: config,
	}
}

func (ds *SweBenchDataService) Load(ctx context.Context, search DataSearch) *Datasource {
	if search.Key == "" {
		return nil
	}
	if val, exist := ds.mappings[search.Key]; exist {
		return &Datasource{
			Image:    val,
			Instance: search.Key,
		}
	} else {
		return nil
	}
}

type CompositeDataService struct {
	registry map[string]DatesetService
}

func NewCompositeDataService() *CompositeDataService {
	registry := make(map[string]DatesetService)
	registry["swebench"] = NewSweBenchDataService()
	return &CompositeDataService{
		registry: registry,
	}
}

func (ds *CompositeDataService) Load(ctx context.Context, search DataSearch) *Datasource {
	scenario := search.Scenario
	if loader, exist := ds.registry[scenario]; exist {
		return loader.Load(ctx, search)
	} else {
		return nil
	}
}
