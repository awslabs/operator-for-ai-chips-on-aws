# AWS Neuron Operator Deployment

Three installation methods for deploying the AWS Neuron Operator with prerequisites (NFD, KMM) on OpenShift.

## Prerequisites

- Red Hat OpenShift Service on AWS (ROSA) with HCP 4.19+, or OpenShift Container Platform (OCP) 4.19+
- `oc` CLI with cluster-admin access
- Cluster with Neuron instance nodes (inf1, inf2, trn1, trn1n)
- Helm 3.x (for install script and Helm options)

## Option 1: ArgoCD (OpenShift GitOps) — recommended

Uses sync waves to install operators in the correct order. Fully declarative and idempotent.

### Install OpenShift GitOps (if not already installed)

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

Grant the ArgoCD controller cluster-admin (required to create namespaces and operator subscriptions):

```bash
oc adm policy add-cluster-role-to-user cluster-admin \
  system:serviceaccount:openshift-gitops:openshift-gitops-argocd-application-controller
```

### Deploy with defaults

```bash
oc apply -f https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/main/deploy/argocd/application.yaml
```

### Deploy with overrides

To skip prerequisites or override image versions, apply the Application with `helm.parameters`:

```bash
oc apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: aws-neuron-operator
  namespace: openshift-gitops
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: https://github.com/awslabs/operator-for-ai-chips-on-aws.git
    targetRevision: main
    path: deploy/helm/aws-neuron-operator
    helm:
      parameters:
        - name: nfd.enabled
          value: "false"
        - name: kmm.enabled
          value: "false"
  destination:
    server: https://kubernetes.default.svc
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    retry:
      limit: 10
      backoff:
        duration: 30s
        factor: 2
        maxDuration: 5m
    syncOptions:
      - SkipDryRunOnMissingResource=true
EOF
```

Available parameters (see `deploy/helm/aws-neuron-operator/values.yaml` for defaults):

| Parameter | Description |
|---|---|
| `nfd.enabled` | Install Node Feature Discovery (default: `true`) |
| `kmm.enabled` | Install Kernel Module Management (default: `true`) |
| `deviceConfig.driversVersion` | Neuron kernel module version |
| `deviceConfig.devicePluginVersion` | Neuron device plugin version |
| `deviceConfig.schedulerVersion` | kube-scheduler version |
| `deviceConfig.schedulerExtensionVersion` | Neuron scheduler extension version |
| `deviceConfig.monitorVersion` | Neuron monitor version |

### Sync waves

- Wave 0: Namespace + NFD subscription
- Wave 1: KMM subscription
- Wave 2: NFD instance + NFD rule
- Wave 3: Neuron operator (CatalogSource + Subscription)
- Wave 4: DeviceConfig

## Option 2: Helm

```bash
helm install aws-neuron-operator deploy/helm/aws-neuron-operator/

# With overrides
helm install aws-neuron-operator deploy/helm/aws-neuron-operator/ \
  --set nfd.enabled=false \
  --set kmm.enabled=false \
  --set deviceConfig.devicePluginVersion=2.30.0.0
```

## Option 3: Install Script

For clusters without ArgoCD or Helm. Handles ordering via wait loops.

```bash
# Full install (includes NFD + KMM)
./deploy/install.sh

# Skip NFD if already installed
./deploy/install.sh --skip-nfd

# Skip both prereqs
./deploy/install.sh --skip-nfd --skip-kmm
```

## Verify

```bash
oc get csv -n ai-operator-on-aws
oc get pods -n ai-operator-on-aws
oc get nodes -l feature.node.kubernetes.io/aws-neuron=true
```
