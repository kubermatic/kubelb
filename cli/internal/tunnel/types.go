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
	"fmt"
	"math/rand"
	"time"
)

// CreateOptions defines options for creating a tunnel
type CreateOptions struct {
	Name     string
	Port     int
	Hostname string
	Wait     bool
	Connect  bool
	Output   string
}

// GenerateTunnelName generates a random tunnel name for the expose command
func GenerateTunnelName() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	adjectives := []string{"quick", "brave", "clever", "bright", "swift", "gentle", "happy", "calm", "bold", "smart"}
	nouns := []string{"tunnel", "bridge", "portal", "gateway", "passage", "conduit", "channel", "link", "path", "route"}

	adjective := adjectives[r.Intn(len(adjectives))]
	noun := nouns[r.Intn(len(nouns))]
	number := r.Intn(1000)

	return fmt.Sprintf("%s-%s-%d", adjective, noun, number)
}
