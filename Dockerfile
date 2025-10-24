# Build the manager binary
FROM golang:1.24 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
# ENV GOPROXY=direct
# ENV GOSUMDB=off
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY Makefile Makefile

# Build
RUN make manager

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.6

# Update packages to get latest security fixes
RUN microdnf update -y && microdnf clean all

WORKDIR /
COPY --from=builder /workspace/manager .

RUN ["groupadd", "--system", "-g", "201", "aws-neuron"]
RUN ["useradd", "--system", "-u", "201", "-g", "201", "-s", "/sbin/nologin", "aws-neuron"]

USER 201:201

ENTRYPOINT ["/manager"]
