# Copyright 2025 The KubeLB Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

BINARY_NAME := kubelb
BUILD_DIR := bin
MAIN_PACKAGE := .

GIT_VERSION := $(shell git describe --tags --match "v*" --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X 'k8c.io/kubelb/cli/cmd.gitVersion=$(GIT_VERSION)' \
	-X 'k8c.io/kubelb/cli/cmd.gitCommit=$(GIT_COMMIT)' \
	-X 'k8c.io/kubelb/cli/cmd.buildDate=$(BUILD_DATE)'"

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

lint: ## Run golangci-lint against code.
	golangci-lint run -v

check-dependencies: ## Verify go.mod.
	go mod verify

go-mod-tidy:
	go mod tidy

.PHONY: all
all: build

.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -mod=mod $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

.PHONY: test
test:
	@echo "Running tests..."
	go test ./...

.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) $(MAIN_PACKAGE)

.PHONY: run-version
run-version: build
	@echo "Running version command..."
	./$(BUILD_DIR)/$(BINARY_NAME) version

update-docs:
	@echo "Updating docs..."
	./$(BUILD_DIR)/$(BINARY_NAME) docs --hugo --output=./docs/cli
