# Build the proxy-daemon binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY . .
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
ENV GOPROXY=https://goproxy.cn

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -a -o proxy-daemon ./cmd/proxy-daemon

# Use distroless as minimal base image to package the proxy-daemon binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM busybox:latest
WORKDIR /
COPY --from=builder /workspace/proxy-daemon .

ENTRYPOINT ["/proxy-daemon"]
