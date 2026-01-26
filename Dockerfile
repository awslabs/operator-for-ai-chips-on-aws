# Build the manager binary
FROM golang:1.24 AS builder
ARG TARGETOS
ARG TARGETARCH

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
COPY Makefile Makefile

# Build
RUN make manager

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.7

ARG VERSION=0.1.3

# Required labels for Red Hat certification
LABEL name="AWS Neuron Operator" \
      maintainer="https://github.com/awslabs/operator-for-ai-chips-on-aws/issues" \
      vendor="Amazon Web Services" \
      version="${VERSION}" \
      release="1" \
      summary="Operator for AWS Neuron devices on OpenShift" \
      description="Automates enabling AWS Neuron devices (Inferentia/Trainium) on OpenShift clusters. Deploys kernel modules via KMM, device plugins, custom scheduler, and node metrics."

WORKDIR /
COPY --from=builder /workspace/manager .

# Add /licenses directory for Red Hat certification
RUN mkdir -p /licenses
COPY LICENSE /licenses/
COPY NOTICE /licenses/

RUN ["groupadd", "--system", "-g", "201", "aws-neuron"]
RUN ["useradd", "--system", "-u", "201", "-g", "201", "-s", "/sbin/nologin", "aws-neuron"]

USER 201:201

ENTRYPOINT ["/manager"]
