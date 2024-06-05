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

FROM docker.io/golang:1.22.2 as builder

WORKDIR /go/src/k8c.io/kubelb
COPY . .
RUN make build-kubelb

FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder \
    /go/src/k8c.io/kubelb/bin/kubelb \
    /usr/local/bin/

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/kubelb"]
