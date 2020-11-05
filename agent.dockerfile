# Build the agent binary
FROM golang:1.13 as builder

WORKDIR /agent
# Copy the Go Modules manifests
COPY agent/go.mod go.mod
COPY agent/go.sum go.sum

COPY manager/go.mod /manager/go.mod
COPY manager/go.sum /manager/go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY agent/main.go main.go
COPY agent/pkg/ pkg/

COPY manager/main.go /manager/main.go
COPY manager/pkg/ /manager/pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o agent main.go

# Use distroless as minimal base image to package the agent binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /agent/agent .
USER nonroot:nonroot

ENTRYPOINT ["/agent"]
