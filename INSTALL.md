# AWS Neuron Operator Installation Guide

## Quick Install (Recommended)

### Prerequisites
- OpenShift 4.19+
- Install NFD and KMM operators from OperatorHub
- Apply default NFD CR from OLM UI

### Direct Install
```bash
# Install the operator
kubectl apply -f https://github.com/awslabs/operator-for-ai-chips-on-aws/releases/latest/download/aws-neuron-operator.yaml

# Create DeviceConfig
kubectl apply -f https://github.com/awslabs/operator-for-ai-chips-on-aws/releases/latest/download/deviceconfig-sample.yaml
```

### OLM Install
```bash
# Apply NFD rule (required for OLM)
kubectl apply -f https://github.com/awslabs/operator-for-ai-chips-on-aws/releases/latest/download/nfd-rule.yaml

# Install via OperatorHub or CatalogSource
# See main README for detailed OLM instructions
```

## Version-Specific Install
Replace `latest` with specific version (e.g., `v0.1.0`):
```bash
kubectl apply -f https://github.com/awslabs/operator-for-ai-chips-on-aws/releases/download/v0.1.0/aws-neuron-operator.yaml
```