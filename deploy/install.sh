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

# 1. Namespace + OperatorGroup
echo "Creating namespace..."
oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ai-operator-on-aws
  labels:
    control-plane: controller-manager
    security.openshift.io/scc.podSecurityLabelSync: "true"
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: aws-neuron-operator
  namespace: ai-operator-on-aws
spec: {}
EOF

# 2. NFD
if [[ "$SKIP_NFD" == "false" ]]; then
  echo "Installing Node Feature Discovery..."
  oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: openshift-nfd
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: openshift-nfd
  namespace: openshift-nfd
spec:
  targetNamespaces:
    - openshift-nfd
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: nfd
  namespace: openshift-nfd
spec:
  channel: stable
  name: nfd
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF
  wait_for_csv openshift-nfd nfd
  wait_for_crd nodefeaturerules.nfd.openshift.io

  oc apply -f - <<EOF
apiVersion: nfd.openshift.io/v1
kind: NodeFeatureDiscovery
metadata:
  name: nfd-instance
  namespace: openshift-nfd
spec: {}
EOF
else
  echo "Skipping NFD installation (--skip-nfd)"
fi

# 3. NFD Rule (always needed)
echo "Applying NFD rule for Neuron devices..."
oc apply -f - <<EOF
apiVersion: nfd.openshift.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: neuron-nfd-rule
  namespace: ai-operator-on-aws
spec:
  rules:
    - name: neuron-device
      labels:
        feature.node.kubernetes.io/aws-neuron: "true"
      matchAny:
        - matchFeatures:
            - feature: pci.device
              matchExpressions:
                vendor: {op: In, value: ["1d0f"]}
                device: {op: In, value: ["7064","7065","7066","7067","7164","7264","7364"]}
EOF

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
oc apply -f - <<EOF
apiVersion: k8s.aws/v1beta1
kind: DeviceConfig
metadata:
  name: neuron
  namespace: ai-operator-on-aws
spec:
  driversImage: public.ecr.aws/os-partners/neuron-openshift/neuron-kernel-module:2.25.4.0
  devicePluginImage: public.ecr.aws/neuron/neuron-device-plugin:2.29.16.0
  customSchedulerImage: public.ecr.aws/eks-distro/kubernetes/kube-scheduler:v1.32.9-eks-1-32-24
  schedulerExtensionImage: public.ecr.aws/neuron/neuron-scheduler:2.29.16.0
  nodeMetricsImage: public.ecr.aws/neuron/neuron-monitor:1.3.0
  selector:
    feature.node.kubernetes.io/aws-neuron: "true"
EOF

echo ""
echo "✓ AWS Neuron Operator installed successfully"
echo "  Verify: oc get pods -n ai-operator-on-aws"
