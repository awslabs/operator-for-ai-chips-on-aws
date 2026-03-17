# AWS Neuron Operator Deployment

Two installation methods for deploying the AWS Neuron Operator with prerequisites (NFD, KMM) on OpenShift.

## Prerequisites

- OpenShift 4.19+
- `oc` CLI with cluster-admin access
- Cluster with Neuron instance nodes (inf1, inf2, trn1, trn1n)

## Option 1: ArgoCD (OpenShift GitOps) — recommended

Uses sync waves to install operators in the correct order. Fully declarative and idempotent.

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

Sync waves handle ordering:
- Wave 0: Namespace
- Wave 1: NFD + KMM subscriptions
- Wave 2: NFD instance + NFD rule
- Wave 3: Neuron operator subscription
- Wave 4: DeviceConfig

## Option 2: Install Script

For clusters without ArgoCD. Handles ordering via wait loops.

```bash
# Full install (includes NFD + KMM)
./deploy/install.sh

# Skip NFD if already installed
./deploy/install.sh --skip-nfd

# Skip both prereqs
./deploy/install.sh --skip-nfd --skip-kmm
```

## Skipping Prerequisites

If NFD or KMM are already installed in your cluster:

- **ArgoCD**: set `nfd.enabled: false` and/or `kmm.enabled: false` in `deploy/argocd/application.yaml`
- **Script**: use `--skip-nfd` and/or `--skip-kmm` flags

## Verify

```bash
oc get pods -n ai-operator-on-aws
oc get nodes -l feature.node.kubernetes.io/aws-neuron=true
```
