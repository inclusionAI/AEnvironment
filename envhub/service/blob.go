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

// BlobAccess describes the access style of a presigned URL.
type BlobAccess string

const (
	BlobRead  BlobAccess = "read"
	BlobWrite BlobAccess = "write"
)

// BlobStorage abstracts object storage that can produce time-bound presigned URLs for an env
// artifact key. The default open-source implementation is OssStorage (Aliyun OSS); internal
// builds may register alternatives (e.g. DDSOSS) via RegisterBlobStorage.
type BlobStorage interface {
	PresignEnv(envKey string, access BlobAccess) (string, error)
}
