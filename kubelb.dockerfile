

FROM docker.io/golang:1.25.5 AS builder

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
COPY VERSION VERSION
COPY Makefile Makefile

# Pass build arguments to make command
RUN GIT_VERSION="${GIT_VERSION}" \
    GIT_COMMIT="${GIT_COMMIT}" \
    BUILD_DATE="${BUILD_DATE}" \
    make build-kubelb

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/bin/kubelb .
USER 65532:65532

ENTRYPOINT ["/kubelb"]
