#!/bin/bash
set -e

VERSION=${1:-"latest"}
IMG=${2:-"public.ecr.aws/os-partners/neuron-openshift/operator:v${VERSION}"}
TEST_MODE=${3:-""}

echo "Generating manifests for version: $VERSION"
echo "Operator image: $IMG"

# Determine if we're in test mode
if [[ "$TEST_MODE" == "--test-mode" ]] || [[ "$IMG" == *"582767206473.dkr.ecr.us-east-1.amazonaws.com"* ]]; then
    echo "Running in test mode - using private registry"
    IS_TEST_MODE=true
    REGISTRY_BASE="582767206473.dkr.ecr.us-east-1.amazonaws.com/neuron-openshift"
else
    echo "Running in production mode - using public registry"
    IS_TEST_MODE=false
    REGISTRY_BASE="public.ecr.aws/os-partners/neuron-openshift"
fi

# Generate manifests
make manifests

# Create release directory
mkdir -p release/$VERSION

# Generate direct deploy manifests
cd config/manager && ../../bin/kustomize edit set image controller=$IMG
cd ../..

# Build the manifests
if ! bin/kustomize build config/default > release/$VERSION/aws-neuron-operator.yaml; then
    echo "Error: Failed to generate aws-neuron-operator.yaml"
    exit 1
fi

# Validate the generated manifest
if [[ ! -s "release/$VERSION/aws-neuron-operator.yaml" ]]; then
    echo "Error: Generated aws-neuron-operator.yaml is empty"
    exit 1
fi

echo "✓ Generated aws-neuron-operator.yaml"

# Generate NFD rule (for OLM installs)
if [[ -f "config/nfd/nfd-rule.yaml" ]]; then
    cp config/nfd/nfd-rule.yaml release/$VERSION/nfd-rule.yaml
    echo "✓ Generated nfd-rule.yaml"
else
    echo "Warning: config/nfd/nfd-rule.yaml not found, creating placeholder"
    cat > release/$VERSION/nfd-rule.yaml << EOF
# NFD rule placeholder - replace with actual NFD configuration
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: aws-neuron
spec:
  rules:
  - name: "aws-neuron"
    labels:
      "feature.node.kubernetes.io/aws-neuron": "true"
    matchFeatures:
    - feature: pci.device
      matchExpressions:
      - key: vendor
        op: In
        value: ["1d0f"]
EOF
fi

# Generate sample DeviceConfig with appropriate registry
if [[ "$IS_TEST_MODE" == "true" ]]; then
    # Test mode - use private registry images
    cat > release/$VERSION/deviceconfig-sample.yaml << EOF
apiVersion: k8s.aws/v1alpha1
kind: DeviceConfig
metadata:
  name: neuron-test
  namespace: ai-operator-on-aws
spec:
  driversImage: ${REGISTRY_BASE}/neuron-kernel-module:${VERSION}  # actual pull at runtime will use <image>-\$KERNEL_VERSION
  devicePluginImage: public.ecr.aws/neuron/neuron-device-plugin:2.29.16.0
  customSchedulerImage: public.ecr.aws/eks-distro/kubernetes/kube-scheduler:v1.32.9-eks-1-32-24
  schedulerExtensionImage: public.ecr.aws/neuron/neuron-scheduler:2.29.16.0
  nodeMetricsImage: public.ecr.aws/neuron/neuron-monitor:1.3.0
  imageRepoSecret:
    name: ecr-secret
  selector:
    feature.node.kubernetes.io/aws-neuron: "true"
EOF
else
    # Production mode - use public registry images
    cat > release/$VERSION/deviceconfig-sample.yaml << EOF
apiVersion: k8s.aws/v1alpha1
kind: DeviceConfig
metadata:
  name: neuron
  namespace: ai-operator-on-aws
spec:
  driversImage: public.ecr.aws/os-partners/neuron-openshift/neuron-kernel-module:2.25.4.0  # actual pull at runtime will use <image>-\$KERNEL_VERSION
  devicePluginImage: public.ecr.aws/neuron/neuron-device-plugin:2.29.16.0
  customSchedulerImage: public.ecr.aws/eks-distro/kubernetes/kube-scheduler:v1.32.9-eks-1-32-24
  schedulerExtensionImage: public.ecr.aws/neuron/neuron-scheduler:2.29.16.0
  nodeMetricsImage: public.ecr.aws/neuron/neuron-monitor:1.3.0
  imageRepoSecret:
    name: ecr-secret
  selector:
    feature.node.kubernetes.io/aws-neuron: "true"
EOF
fi

echo "✓ Generated deviceconfig-sample.yaml"

# Validate all generated files
for file in aws-neuron-operator.yaml nfd-rule.yaml deviceconfig-sample.yaml; do
    if [[ ! -f "release/$VERSION/$file" ]]; then
        echo "Error: Failed to generate $file"
        exit 1
    fi
    
    if [[ ! -s "release/$VERSION/$file" ]]; then
        echo "Error: Generated $file is empty"
        exit 1
    fi
done

echo "✓ All manifest files validated successfully"
echo "Manifests generated in release/$VERSION/"
echo "  - aws-neuron-operator.yaml ($(wc -l < release/$VERSION/aws-neuron-operator.yaml) lines)"
echo "  - nfd-rule.yaml ($(wc -l < release/$VERSION/nfd-rule.yaml) lines)"
echo "  - deviceconfig-sample.yaml ($(wc -l < release/$VERSION/deviceconfig-sample.yaml) lines)"