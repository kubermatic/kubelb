# Copyright 2020 The KubeLB Authors.
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

FROM docker.io/golang:1.26.2@sha256:fcdb3e42c5544e9682a635771eac76a698b66de79b1b50ec5b9ce5c5f14ad775 AS builder

ARG GIT_VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/
COPY Makefile Makefile

# Pass build arguments to make command
RUN GIT_VERSION="${GIT_VERSION}" \
    GIT_COMMIT="${GIT_COMMIT}" \
    BUILD_DATE="${BUILD_DATE}" \
    make build-ccm

FROM gcr.io/distroless/static:nonroot@sha256:88a46f645e304fc0dcfbdacdfa338ce02d9890df5f936872243d553278deae92
WORKDIR /
COPY --from=builder /workspace/bin/ccm .
USER 65532:65532

ENTRYPOINT ["/ccm"]
