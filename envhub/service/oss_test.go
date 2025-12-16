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

import "testing"

func TestOssStorage(t *testing.T) {
	config := LoadOssConfigFromEnv()
	ossStorage, err := NewOssStorage(config)
	if err != nil {
		t.Fatal(err)
	}
	if ossStorage == nil {
		t.Skip("OSS storage is not configured, skipping test")
		return
	}
	url, err := ossStorage.PresignEnv("mucustom-0.0.1.tar", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(url)
}

func TestOssRead(t *testing.T) {
	val := readConfig("/datasource/swe-verify-images.json")
	t.Log(val)
}
