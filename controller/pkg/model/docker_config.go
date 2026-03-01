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

package model

// DockerConfig represents Docker engine configuration
type DockerConfig struct {
	// Host is the Docker daemon address (unix:///var/run/docker.sock or tcp://host:2376)
	Host string `json:"host,omitempty"`

	// TLSVerify enables TLS verification for remote Docker daemon
	TLSVerify bool `json:"tlsVerify,omitempty"`

	// CertPath is the path to TLS certificates directory
	CertPath string `json:"certPath,omitempty"`

	// NetworkMode specifies the default network mode (bridge, host, none, custom)
	NetworkMode string `json:"networkMode,omitempty"`

	// DefaultNetwork is the custom network name to use
	DefaultNetwork string `json:"defaultNetwork,omitempty"`

	// ComposeEnabled enables Docker Compose support
	ComposeEnabled bool `json:"composeEnabled,omitempty"`

	// DefaultCPU is the default CPU limit (e.g., "1.0C", "2C")
	DefaultCPU string `json:"defaultCPU,omitempty"`

	// DefaultMemory is the default memory limit (e.g., "2Gi", "512Mi")
	DefaultMemory string `json:"defaultMemory,omitempty"`
}

// DefaultDockerConfig returns the default Docker configuration
func DefaultDockerConfig() *DockerConfig {
	return &DockerConfig{
		Host:           "unix:///var/run/docker.sock",
		TLSVerify:      false,
		CertPath:       "",
		NetworkMode:    "bridge",
		DefaultNetwork: "",
		ComposeEnabled: true,
		DefaultCPU:     "1.0C",
		DefaultMemory:  "2Gi",
	}
}
