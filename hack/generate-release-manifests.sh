#!/bin/bash
# Generate release artifacts for GitHub release attachment.
# Usage: ./hack/generate-release-manifests.sh <version>
set -euo pipefail

VERSION=${1:?Usage: $0 <version>}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CHART_DIR="$ROOT_DIR/deploy/helm/aws-neuron-operator"
RELEASE_DIR="$ROOT_DIR/release/$VERSION"

mkdir -p "$RELEASE_DIR"

# 1. Operator manifest via kustomize
cd "$ROOT_DIR/config/manager"
../../bin/kustomize edit set image controller="public.ecr.aws/os-partners/neuron-openshift/operator:v${VERSION}"
cd "$ROOT_DIR"
bin/kustomize build config/default > "$RELEASE_DIR/aws-neuron-operator.yaml"
echo "✓ aws-neuron-operator.yaml"

# 2. NFD rule and DeviceConfig from Helm chart (single source of truth)
helm template release "$CHART_DIR" -s templates/nfd-rule.yaml | grep -v '^#' > "$RELEASE_DIR/nfd-rule.yaml"
helm template release "$CHART_DIR" -s templates/deviceconfig.yaml | grep -v '^#' > "$RELEASE_DIR/deviceconfig-sample.yaml"
echo "✓ nfd-rule.yaml"
echo "✓ deviceconfig-sample.yaml"

echo "Artifacts in $RELEASE_DIR/"
