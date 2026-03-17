# AWS Neuron Operator Deployment

Three installation methods for deploying the AWS Neuron Operator with prerequisites (NFD, KMM) on OpenShift.

## Prerequisites

- OpenShift 4.19+
- `oc` CLI with cluster-admin access
- Cluster with Neuron instance nodes (inf1, inf2, trn1, trn1n)

## Option 1: Shell Script (simplest)

```bash
# Full install (includes NFD + KMM)
./deploy/install.sh

# Skip NFD if already installed
./deploy/install.sh --skip-nfd

# Skip both prereqs
./deploy/install.sh --skip-nfd --skip-kmm
```

## Option 2: Helm

```bash
# Full install
helm install neuron-operator deploy/helm/aws-neuron-operator/

# Skip prereqs
helm install neuron-operator deploy/helm/aws-neuron-operator/ \
  --set nfd.enabled=false --set kmm.enabled=false
```

## Option 3: ArgoCD (OpenShift GitOps)

Requires the OpenShift GitOps operator. If not already installed:
```bash
oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-gitops-operator
  namespace: openshift-operators
spec:
  channel: latest
  name: openshift-gitops-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF
```

Then deploy the Neuron operator:
```bash
# Edit deploy/argocd/application.yaml to set nfd.enabled / kmm.enabled as needed
oc apply -f deploy/argocd/application.yaml
```

## Verify

```bash
oc get pods -n ai-operator-on-aws
oc get nodes -l feature.node.kubernetes.io/aws-neuron=true
```
