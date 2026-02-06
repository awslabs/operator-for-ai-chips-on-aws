# Red Hat Preflight Local Testing Guide

## Overview
This guide shows how to run Red Hat preflight certification checks on locally built operator images before pushing to production.

## Prerequisites
- Docker installed and running
- Operator image built locally
- `results/` directory for output

## Steps

### 1. Build Operator Image Locally

```bash
# Vendor dependencies (if behind corporate firewall)
go mod vendor

# Build image for linux/amd64
docker build \
  --platform linux/amd64 \
  --build-arg TARGET=manager \
  --build-arg VERSION=1.0.0 \
  -t neuron-operator:local-test \
  .
```

### 2. Start Local Registry

```bash
# Start local Docker registry
docker run -d -p 5000:5000 --name registry registry:2
```

### 3. Push Image to Local Registry

```bash
# Tag image for local registry
docker tag neuron-operator:local-test localhost:5000/neuron-operator:local-test

# Push to local registry
docker push localhost:5000/neuron-operator:local-test
```

### 4. Run Preflight Checks

```bash
# Create results directory
mkdir -p results

# Run preflight
docker run -it --rm \
  --add-host=host.docker.internal:host-gateway \
  -v $(pwd)/results:/artifacts \
  quay.io/opdev/preflight:stable check container --platform=linux/amd64 --insecure \
  host.docker.internal:5000/neuron-operator:local-test
```

**Key flags**:
- `--add-host=host.docker.internal:host-gateway`: Allows preflight container to access host's localhost
- `--insecure`: Required for local registry without TLS
- `--platform=linux/amd64`: Specifies architecture to test

### 5. Review Results

```bash
# View results
cat results/results.json | jq '.passed'
cat results/results.json | jq '.failed'

# View detailed report
cat results/results.json | jq
```

## Cleanup

```bash
# Stop and remove local registry
docker stop registry
docker rm registry

# Remove test images
docker rmi localhost:5000/neuron-operator:local-test
docker rmi neuron-operator:local-test
```

## Common Issues

### Issue: "connection refused" to localhost:5000
**Solution**: Use `host.docker.internal:5000` with `--add-host` flag

### Issue: "go mod download" fails during build
**Solution**: Run `go mod vendor` first, then build uses local vendor/ directory

### Issue: Platform mismatch warning
**Solution**: This is expected when building linux/amd64 on Mac ARM. Image still works correctly.

## Red Hat Certification Requirements

Your image must pass these checks:
- ✅ **HasLicense**: `/licenses/` directory with LICENSE and NOTICE files
- ✅ **HasUniqueTag**: Image has specific version tag (not `latest`)
- ✅ **LayerCountAcceptable**: <40 layers
- ✅ **RunAsNonRoot**: Runs as non-root user (UID 201)
- ✅ **BasedOnUbi**: Uses Red Hat UBI base image
- ✅ **HasNoProhibitedLabels**: No Red Hat trademarks in labels
- ✅ **HasRequiredLabel**: All required labels present (name, vendor, version, etc.)

## Next Steps

After local testing passes:
1. Push image to ECR Public (`public.ecr.aws/os-partners/neuron-openshift/operator`)
2. Register component in Red Hat Partner Connect
3. Submit for official certification
4. Monitor certification status in Partner Connect portal
