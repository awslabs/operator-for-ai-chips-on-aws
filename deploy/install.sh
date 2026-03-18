#!/usr/bin/env bash
# Install AWS Neuron Operator with prerequisites on OpenShift.
# Usage:
#   ./install.sh                          # full install
#   ./install.sh --skip-nfd               # skip NFD (already installed)
#   ./install.sh --skip-kmm               # skip KMM (already installed)
#   ./install.sh --skip-nfd --skip-kmm    # operator only
set -euo pipefail

SKIP_NFD=false
SKIP_KMM=false
EXTRA_ARGS=()

for arg in "$@"; do
  case $arg in
    --skip-nfd) SKIP_NFD=true ;;
    --skip-kmm) SKIP_KMM=true ;;
    *) echo "Unknown option: $arg"; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/helm/aws-neuron-operator"

command -v helm &>/dev/null || { echo "Error: helm is required"; exit 1; }
command -v oc &>/dev/null || { echo "Error: oc is required"; exit 1; }

[[ "$SKIP_NFD" == "true" ]] && EXTRA_ARGS+=(--set nfd.enabled=false)
[[ "$SKIP_KMM" == "true" ]] && EXTRA_ARGS+=(--set kmm.enabled=false)

wait_for_csv() {
  local ns=$1 name=$2
  echo "Waiting for $name CSV to succeed..."
  until oc get csv -n "$ns" 2>/dev/null | grep "$name" | grep -q Succeeded; do sleep 10; done
  echo "✓ $name is ready"
}

wait_for_crd() {
  local crd=$1
  echo "Waiting for CRD $crd..."
  until oc get crd "$crd" &>/dev/null; do sleep 5; done
  echo "✓ CRD $crd available"
}

render() {
  helm template aws-neuron-operator "$CHART_DIR" "${EXTRA_ARGS[@]}" -s "templates/$1"
}

# Wave 0: Namespace
echo "Creating namespace..."
render namespace.yaml | oc apply -f -

# Wave 0-1: NFD
if [[ "$SKIP_NFD" == "false" ]]; then
  echo "Installing Node Feature Discovery..."
  render nfd-subscription.yaml | oc apply -f -
  wait_for_csv openshift-nfd nfd
  wait_for_crd nodefeaturerules.nfd.openshift.io
  echo "Creating NFD instance..."
  render nfd-instance.yaml | oc apply -f -
else
  echo "Skipping NFD (--skip-nfd)"
fi

# Wave 1: KMM
if [[ "$SKIP_KMM" == "false" ]]; then
  echo "Installing Kernel Module Management..."
  render kmm-subscription.yaml | oc apply -f -
  wait_for_csv openshift-operators kernel-module-management
else
  echo "Skipping KMM (--skip-kmm)"
fi

# Wave 2: NFD Rule
echo "Applying NFD rule..."
render nfd-rule.yaml | oc apply -f -

# Wave 3: Neuron Operator
echo "Installing AWS Neuron Operator..."
render neuron-subscription.yaml | oc apply -f -
wait_for_csv ai-operator-on-aws aws-neuron-operator
wait_for_crd deviceconfigs.k8s.aws

# Wave 4: DeviceConfig
echo "Creating DeviceConfig..."
render deviceconfig.yaml | oc apply -f -

echo ""
echo "✓ AWS Neuron Operator installed successfully"
echo "  Verify: oc get pods -n ai-operator-on-aws"
