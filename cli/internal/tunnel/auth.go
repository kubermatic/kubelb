/*
Copyright 2025 The KubeLB Authors.

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

package tunnel

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	kubelbce "k8c.io/kubelb/api/ee/kubelb.k8c.io/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Auth holds tunnel authentication information
type Auth struct {
	Token       string
	TokenExpiry time.Time
}

// LoadTunnelAuth reads authentication token from SyncSecret
func LoadTunnelAuth(ctx context.Context, kubeClient client.Client, namespace, tunnelName string) (*Auth, error) {
	syncSecret := &kubelbce.SyncSecret{}
	err := kubeClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf("tunnel-auth-%s", tunnelName),
	}, syncSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to read tunnel auth: %w", err)
	}

	// Token should be used as-is (base64 string for HTTP headers)
	// TokenExpiry needs to be decoded if base64 encoded
	tokenData := syncSecret.Data["token"]
	if len(tokenData) == 0 {
		return nil, fmt.Errorf("no token found in sync secret")
	}
	token := strings.TrimSpace(string(tokenData))
	if token == "" {
		return nil, fmt.Errorf("token is empty")
	}

	expiryStr, err := decodeBase64Field(syncSecret.Data["tokenExpiry"], "tokenExpiry")
	if err != nil {
		return nil, err
	}

	// Parse token expiry timestamp
	tokenExpiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token expiry: %w", err)
	}

	// Check token freshness
	if time.Now().After(tokenExpiry) {
		return nil, fmt.Errorf("token expired at %v", tokenExpiry)
	}

	return &Auth{
		Token:       token,
		TokenExpiry: tokenExpiry,
	}, nil
}

// decodeBase64Field decodes base64-encoded data from SyncSecret with fallback
func decodeBase64Field(data []byte, fieldName string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("no %s found in sync secret", fieldName)
	}

	rawData := strings.TrimSpace(string(data))
	if rawData == "" {
		return "", fmt.Errorf("%s is empty", fieldName)
	}

	// Try base64 decoding first (expected case)
	if decoded, err := base64.StdEncoding.DecodeString(rawData); err == nil {
		result := strings.TrimSpace(string(decoded))
		if result != "" {
			return result, nil
		}
	}
	return rawData, nil
}
