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

package clients

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	SIGN_FORMAT = "AccessKey=%s&timestamp=%s"
)

type ACIClient struct {
	host     string
	access   string
	secret   string
	project  int
	template int
}

func NewACIClient(access string, secret string) *ACIClient {
	return &ACIClient{
		host:     "devapi.alipay.com",
		access:   access,
		secret:   secret,
		project:  185100010,
		template: 156300005,
	}
}

type RunRequest struct {
	ProjectId          int                      `json:"projectId"`
	Branch             string                   `json:"branch"`
	PipelineTemplateId int                      `json:"pipelineTemplateId"`
	Parameters         []map[string]interface{} `json:"parameters"`
}

func (c *ACIClient) make() RunRequest {
	return RunRequest{
		ProjectId: c.project,
		Parameters: []map[string]interface{}{
			{"key": "name", "value": "test"},
			{"key": "instanceId", "value": "version"},
		},
		Branch:             "master",
		PipelineTemplateId: c.template,
	}
}

// PKCS5Padding performs PKCS5 padding on data
func PKCS5Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

// AESECBEncrypt encrypts string using AES/ECB/PKCS5Padding
// src: string to encrypt
// key: key (must be 16 characters)
// Returns: hexadecimal encrypted result
func AESECBEncrypt(src, key string) (string, error) {
	if len(key) != 16 {
		return "", errors.New("Key must be 16 characters long")
	}

	// Use UTF-8 encoding for key and plaintext
	keyBytes := []byte(key)
	srcBytes := []byte(src)

	// PKCS5 padding
	paddedData := PKCS5Padding(srcBytes, aes.BlockSize)

	// Create AES cipher
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	// ECB mode encryption
	encrypted := make([]byte, len(paddedData))
	size := block.BlockSize()
	for i := 0; i < len(paddedData); i += size {
		block.Encrypt(encrypted[i:i+size], paddedData[i:i+size])
	}

	// Convert to hexadecimal string
	return hex.EncodeToString(encrypted), nil
}

func (c *ACIClient) Sign(timestamp string) string {
	// 1. Build string to sign
	signStr := fmt.Sprintf(SIGN_FORMAT, c.access, timestamp)
	val, err := AESECBEncrypt(signStr, c.secret)
	if err != nil {
		return ""
	}
	return val
}
func (c *ACIClient) trigger() {
	// Send request
	url := "https://" + c.host + "/aci/api/v1/pipeline"
	jsonData, err := json.Marshal(c.make())
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		panic(err)
	}
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1_000_000)
	signature := c.Sign(timestamp)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AccessKey", c.access)
	req.Header.Set("Timestamp", timestamp)
	req.Header.Set("Signature", signature)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
