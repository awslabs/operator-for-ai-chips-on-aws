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

for arg in "$@"; do
  case $arg in
    --skip-nfd) SKIP_NFD=true ;;
    --skip-kmm) SKIP_KMM=true ;;
    *) echo "Unknown option: $arg"; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# 1. Namespace
echo "Creating namespace..."
oc apply -f "${SCRIPT_DIR}/helm/aws-neuron-operator/templates/namespace.yaml"

# 2. NFD
if [[ "$SKIP_NFD" == "false" ]]; then
  echo "Installing Node Feature Discovery..."
  oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: nfd
  namespace: openshift-operators
spec:
  channel: stable
  name: nfd
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF
  wait_for_csv openshift-operators nfd
  wait_for_crd nodefeaturerules.nfd.openshift.io

  oc apply -f - <<EOF
apiVersion: nfd.openshift.io/v1
kind: NodeFeatureDiscovery
metadata:
  name: nfd-instance
  namespace: openshift-operators
spec: {}
EOF
else
  echo "Skipping NFD installation (--skip-nfd)"
fi

# 3. NFD Rule (always needed)
echo "Applying NFD rule for Neuron devices..."
oc apply -f "${SCRIPT_DIR}/helm/aws-neuron-operator/templates/nfd-rule.yaml"

# 4. KMM
if [[ "$SKIP_KMM" == "false" ]]; then
  echo "Installing Kernel Module Management..."
  oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: kernel-module-management
  namespace: openshift-operators
spec:
  channel: stable
  name: kernel-module-management
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF
  wait_for_csv openshift-operators kernel-module-management
else
  echo "Skipping KMM installation (--skip-kmm)"
fi

# 5. Neuron Operator
echo "Installing AWS Neuron Operator..."
oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: aws-neuron-operator
  namespace: ai-operator-on-aws
spec:
  channel: stable
  name: aws-neuron-operator
  source: certified-operators
  sourceNamespace: openshift-marketplace
EOF
wait_for_csv ai-operator-on-aws aws-neuron-operator
wait_for_crd deviceconfigs.k8s.aws

# 6. DeviceConfig
echo "Creating DeviceConfig..."
oc apply -f "${SCRIPT_DIR}/helm/aws-neuron-operator/templates/deviceconfig.yaml"

echo ""
echo "✓ AWS Neuron Operator installed successfully"
echo "  Verify: oc get pods -n ai-operator-on-aws"
