#!/usr/bin/env bash
# Remove all artifacts created by install.sh.
set -uo pipefail

SKIP_NFD=false
SKIP_KMM=false

for arg in "$@"; do
  case $arg in
    --skip-nfd) SKIP_NFD=true ;;
    --skip-kmm) SKIP_KMM=true ;;
    *) echo "Unknown option: $arg"; exit 1 ;;
  esac
done

NS=aws-neuron-operator

echo "Removing DeviceConfig..."
oc delete deviceconfig neuron -n "$NS" --ignore-not-found

echo "Removing Neuron operator subscription and CSV..."
oc delete subscription aws-neuron-operator -n "$NS" --ignore-not-found
oc delete csv -n "$NS" -l operators.coreos.com/aws-neuron-operator."$NS"= --ignore-not-found 2>/dev/null
oc delete catalogsource aws-neuron-operator -n openshift-marketplace --ignore-not-found

echo "Removing NFD rule..."
oc delete nodefeaturerule neuron-nfd-rule -n "$NS" --ignore-not-found

if [[ "$SKIP_KMM" == "false" ]]; then
  echo "Removing KMM subscription and CSV..."
  oc delete subscription kernel-module-management -n openshift-operators --ignore-not-found
  oc delete csv -n openshift-operators -l operators.coreos.com/kernel-module-management.openshift-operators= --ignore-not-found 2>/dev/null
fi

if [[ "$SKIP_NFD" == "false" ]]; then
  echo "Removing NFD instance..."
  oc delete nodefeaturediscovery nfd-instance -n openshift-nfd --ignore-not-found
  echo "Removing NFD subscription, operator group, and CSV..."
  oc delete subscription nfd -n openshift-nfd --ignore-not-found
  oc delete csv -n openshift-nfd -l operators.coreos.com/nfd.openshift-nfd= --ignore-not-found 2>/dev/null
  oc delete operatorgroup openshift-nfd -n openshift-nfd --ignore-not-found
  echo "Removing openshift-nfd namespace..."
  oc delete namespace openshift-nfd --ignore-not-found
fi

echo "Removing operator group and namespace..."
oc delete operatorgroup aws-neuron-operator -n "$NS" --ignore-not-found
oc delete namespace "$NS" --ignore-not-found

echo ""
echo "✓ All artifacts removed"
