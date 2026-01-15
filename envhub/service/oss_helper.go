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
	"bytes"
	"context"
	"io"
	"log"
	"os"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

// OssConfig holds OSS configuration
type OssConfig struct {
	Endpoint  string
	Bucket    string
	KeyPrefix string
	Region    string
	AccessKey string
	SecretKey string
}

// LoadOssConfigFromEnv loads OSS configuration from environment variables
func LoadOssConfigFromEnv() *OssConfig {
	config := &OssConfig{
		Endpoint:  getEnvOrDefault("OSS_ENDPOINT", ""),
		Bucket:    getEnvOrDefault("OSS_BUCKET", ""),
		KeyPrefix: getEnvOrDefault("OSS_KEY_PREFIX", "aenv"),
		Region:    getEnvOrDefault("OSS_REGION", "cn-hangzhou"),
		AccessKey: getEnvOrDefault("OSS_ACCESS_KEY_ID", ""),
		SecretKey: getEnvOrDefault("OSS_ACCESS_KEY_SECRET", ""),
	}
	return config
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// IsOssConfigured checks if OSS is properly configured
func IsOssConfigured(config *OssConfig) bool {
	// OSS is considered configured if Bucket is set
	// Endpoint and credentials can be optional (using default endpoint or env vars)
	return config.Bucket != ""
}

func makeOssClient(config *OssConfig) (*oss.Client, error) {
	var provider credentials.CredentialsProvider

	// Use static credentials if provided, otherwise fall back to environment variables
	if config.AccessKey != "" && config.SecretKey != "" {
		provider = credentials.NewStaticCredentialsProvider(config.AccessKey, config.SecretKey)
	} else {
		provider = credentials.NewEnvironmentVariableCredentialsProvider()
	}

	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion(config.Region)

	if config.Endpoint != "" {
		cfg = cfg.WithEndpoint(config.Endpoint)
	}

	cfg = cfg.WithSignatureVersion(oss.SignatureVersionV1)
	return oss.NewClient(cfg), nil
}

func makeEnvUploadRequest(config *OssConfig, envName string) *oss.PutObjectRequest {
	objectName := config.KeyPrefix + "/" + envName + ".tar"
	return &oss.PutObjectRequest{
		Bucket: oss.Ptr(config.Bucket),
		Key:    oss.Ptr(objectName),
		Acl:    oss.ObjectACLPublicReadWrite,
	}
}

func makeEnvReadRequest(config *OssConfig, envName string) *oss.GetObjectRequest {
	objectName := config.KeyPrefix + "/" + envName + ".tar"
	return &oss.GetObjectRequest{
		Bucket: oss.Ptr(config.Bucket),
		Key:    oss.Ptr(objectName),
	}
}

// Global OSS client and config (initialized on first use)
var (
	globalOssClient  *oss.Client
	globalOssConfig  *OssConfig
	ossClientInitErr error
)

// initGlobalOssClient initializes global OSS client from environment
func initGlobalOssClient() {
	if globalOssClient != nil || globalOssConfig != nil {
		return
	}
	globalOssConfig = LoadOssConfigFromEnv()
	// Only initialize client if OSS is configured
	if IsOssConfigured(globalOssConfig) {
		globalOssClient, ossClientInitErr = makeOssClient(globalOssConfig)
	}
}

// GetGlobalOssClient returns the global OSS client (initialized from environment)
func GetGlobalOssClient() (*oss.Client, error) {
	initGlobalOssClient()
	return globalOssClient, ossClientInitErr
}

// GetGlobalOssConfig returns the global OSS config (initialized from environment)
func GetGlobalOssConfig() *OssConfig {
	initGlobalOssClient()
	return globalOssConfig
}

func readConfig(key string) string {
	initGlobalOssClient()
	// Return empty string if OSS is not configured or client initialization failed
	if globalOssClient == nil || ossClientInitErr != nil || !IsOssConfigured(globalOssConfig) {
		return ""
	}

	configFile, err := globalOssClient.OpenFile(context.TODO(), globalOssConfig.Bucket, globalOssConfig.KeyPrefix+key)
	if err != nil {
		return ""
	}
	defer func() {
		if closeErr := configFile.Close(); closeErr != nil {
			log.Printf("failed to close config file: %v", closeErr)
		}
	}()

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, configFile)
	if err != nil {
		return ""
	}
	return buffer.String()
}
