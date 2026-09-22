# Performance-optimized Neuron inference on ROSA

A Neuron compile cache on shared EFS storage, plus OpenShift AI and KServe serving,
installed GitOps style. This directory is the entry point, and
[VALIDATION.md](VALIDATION.md) is the reproducible procedure.

A Neuron model is compiled before it can serve, and on a cold start that compile plus the
model download dominates startup. The Neuron SDK caches compiled artifacts on disk, but
in the manifests under `deploy/examples/` that cache is either an `emptyDir`, discarded
when the pod restarts, or a `ReadWriteOnce` gp3 volume that only pods on one node can
share. Putting it on EFS with `ReadWriteMany` means the first pod compiles and every pod
after it reads the result, including pods on other nodes and pods created by scaling out.

## What gets installed

Four Helm charts, one ArgoCD Application each, all generated from
[../argocd/applicationset-performance.yaml](../argocd/applicationset-performance.yaml)
and switched on by a label on one Secret.

| Chart | Label | What it owns |
|---|---|---|
| [../helm/efs-storage/](../helm/efs-storage/) | `enable_storage` | EFS CSI Driver Operator, `ClusterCSIDriver/efs.csi.aws.com`, `StorageClass/efs-sc` |
| [../helm/aws-neuron-operator/](../helm/aws-neuron-operator/) | `enable_neuron_operator` | the Neuron operator with NFD and KMM, and a `DeviceConfig` |
| [../helm/oai-platform/](../helm/oai-platform/) | `enable_oai` | Service Mesh, Serverless and OpenShift AI Subscriptions, and a `DataScienceCluster` with KServe on |
| [../helm/neuron-serving/](../helm/neuron-serving/) | `enable_oai`, suppressed by `enable_serving=false` | one model: `ServingRuntime`, `InferenceService`, compile cache PVC |

Platform and workload are separate charts because their lifecycles differ.
`oai-platform` touches cluster singletons the cluster owner may already have, so it is
configured for adoption. `neuron-serving` is per-model, installable many times over,
and disposable.

## Turning a component off deletes its resources

Read this before flipping a toggle on a cluster that anything else depends on.

Setting a component's label to `false` makes the ArgoCD cluster generator stop matching,
the ApplicationSet deletes that Application, and the Application's resources go with it.
The last step is the default: `spec.syncPolicy.preserveResourcesOnDeletion` on an
ApplicationSet defaults to `false`.

`enable_storage=false` is the sharpest case. `efs-storage` owns
`ClusterCSIDriver/efs.csi.aws.com` and `StorageClass/efs-sc` at cluster scope with
pruning and self-healing on, so turning storage off uninstalls the cluster's EFS CSI
driver and removes a StorageClass that PVCs outside this deployment may reference.

`enable_oai=false` was observed to delete the `neuron-inference`, `openshift-serverless`
and `redhat-ods-operator` namespaces along with the operator Subscriptions in them,
against the single chart that `oai-platform` and `neuron-serving` replaced. An
already-installed ClusterServiceVersion survived, because deleting a Subscription does
not uninstall an operator, so an operator can be left running with nothing declaring it.

`oai-platform` therefore sets `preserveResourcesOnDeletion: true` and its resources now
survive `enable_oai=false`. The other three ApplicationSets do not set it and do delete
their resources; adding that one line to any of them changes that. The
`argocd.argoproj.io/sync-options: Prune=false` annotations in the charts do not help
here, because ArgoCD scopes `Prune` to sync-time pruning of resources no longer in git
and `Delete` to app deletion, and those are separate options.

## Install

Prerequisites: a ROSA cluster with Neuron nodes (inf2, trn1, trn2), `oc` logged in as
cluster-admin, `aws` configured, and `jq`.

**1. Create the AWS prerequisites.** Read the script first, because it creates IAM
roles. Its whole input surface is `--dry-run`, `--yes` to skip the confirmation prompt
for non-interactive runs, `--env-file <path>`, and `--help`. Nothing is required: the
region comes from the cluster's own nodes, which is safer than a default in
`~/.aws/config`, because EFS mount targets have to live in the cluster's region. Copy
[efs.env.example](efs.env.example) to `efs.env` to change a default. Re-running adopts
existing resources, which matters most for the filesystem, since a second one would
silently abandon a warm cache.

```bash
curl -O https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/main/deploy/performance/bootstrap-efs.sh
chmod +x bootstrap-efs.sh
./bootstrap-efs.sh --dry-run   # print every call it would make, change nothing
./bootstrap-efs.sh
```

A script is needed at all because an IAM role whose trust policy references the cluster's
OIDC provider cannot be created from inside the cluster that needs it. The script's
header block lists everything it creates. All of it is named after the cluster's
infrastructure name, and the IAM policy is scoped to this cluster: `CreateAccessPoint`
names this filesystem's ARN, and `DeleteAccessPoint` is conditioned on the two tags the
EFS CSI driver puts on access points, one of which carries the infrastructure name.

**2. Install the OpenShift GitOps operator.** Unchanged from
[../README.md](../README.md).

**3. Apply the deployment.** One file, applied as-is, no editing and no patching.
Replace `main` with a release tag to pin it, since tracking `main` means an upstream
merge can change this cluster. A copy of `bootstrap-efs.sh` downloaded from a release
tag already defaults to that tag, because the release workflow pins
`PERF_REPO_REVISION` in the script and in `efs.env.example`.

```bash
oc apply -f https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/main/deploy/argocd/applicationset-performance.yaml
```

**4. Supply a Hugging Face token.** `neuron-serving` defaults to
`meta-llama/Llama-3.1-8B-Instruct`, which is gated. Nothing in the chart creates a token
Secret and the chart references none until you name one, so create it and set
`model.tokenSecretName` in the chart's values. Public models need no token.

```bash
oc create secret generic hf-token --from-literal=HF_TOKEN=<token> -n neuron-inference
```

## How configuration reaches the charts

Two categories, kept apart on purpose.

**Facts that cannot exist before AWS is provisioned** are the EFS filesystem ID and the
IAM role ARN. `bootstrap-efs.sh` writes those, plus the repo this cluster syncs from, as
annotations on one Secret in `openshift-gitops` that doubles as the ArgoCD cluster entry
(`oc get secret -n openshift-gitops -l neuron_perf=true -o yaml`). Labels on the same
Secret select components. Those two annotations are the only ones any chart receives,
which is why no Application ever needs patching: a patch stores cluster-specific state
on an object git owns, so re-applying the repo or ArgoCD self-healing reverts it.

**Everything else is chart configuration** and lives in the charts' `values.yaml`:
model, tensor parallel size, cache sizes, namespaces, probe timings, images. None of it
is routed through the Secret. A shell script writing model choices into annotations
would put deployment configuration outside version control, where no reviewer sees it
change. That is why neither the script nor `efs.env` has a setting for any of it. To
vary chart configuration per cluster without forking the repo, commit
`values-<cluster>.yaml` beside the chart's `values.yaml`, where `<cluster>` is the
ArgoCD cluster name. Every Application lists both files and sets
`ignoreMissingValueFiles: true`, so the per-cluster file is optional and holds only what
differs.

Toggle components by editing `ENABLE_*` in `efs.env` and re-running the script, which is
the source of truth. Relabelling the Secret directly also works and takes effect within
a minute, but the script re-applies the Secret from `efs.env` on its next run and
reverts the label then. `enable_serving` is the exception: the script does not write it,
so a label is the only way to set it, and the workload follows `enable_oai` when it is
absent.

```bash
sed -i 's/^ENABLE_OAI=.*/ENABLE_OAI=false/' efs.env && ./bootstrap-efs.sh
oc label secret -n openshift-gitops <infra-name>-neuron-perf enable_serving=false --overwrite
```

## Installing without GitOps

Ordinary Helm charts, in this order. Each prints its own next steps.

```bash
helm install efs-storage ../helm/efs-storage \
  --set efs.fileSystemId=fs-... --set efs.roleARN=arn:aws:iam::...:role/...
helm install aws-neuron-operator ../helm/aws-neuron-operator
helm install oai-platform ../helm/oai-platform
helm install llama31-8b ../helm/neuron-serving
```

## What has been validated

Storage and operator layers, end to end on ROSA 4.22.11 across two availability zones.
The script created and then re-adopted every AWS resource with no config file, the
cluster generator produced the Applications, a `ReadWriteMany` PVC bound and released its
EFS access point cleanly, a pod in one availability zone read and appended to an artifact
written by a pod in the other, and the Neuron operator brought up a node advertising
`aws.amazon.com/neuron: 1`. Toggling `enable_oai` off is where the deletion behaviour
above was observed.

The `enable_oai=true` serving path has never been brought up here, so the
`ServingRuntime` and `InferenceService` are unvalidated. **No vLLM or InferenceService
startup time has been measured**, warm or cold, so this deployment claims no startup
improvement. [VALIDATION.md](VALIDATION.md) level 3 is the procedure for producing that
number.

## Known gaps

- **Probe timings are reasoned, not measured.** The startup probe gets a 1230 second
  budget and readiness a 5 second delay, so a warm pod takes traffic without waiting out
  the cold-start budget. Whether Knative's queue-proxy preserves the startup probe end to
  end has not been observed on a running pod.
- **`NEURON_CACHE_URL`**, used by the manifests in `deploy/examples/`, does not appear in
  the AWS Neuron persistent cache documentation. The documented variables are
  `NEURON_COMPILE_CACHE_URL`, which `neuron-serving` sets, and `NEURON_CC_FLAGS
  --cache_dir`, which takes precedence over it.
- **OperatorGroup conflict.** GitOps has to create an OperatorGroup in
  `openshift-cluster-csi-drivers`, verified on a cluster that had none. If yours already
  has one, set `efsCsiOperator.createOperatorGroup=false`.
- **Limits to expect.** At most 1000 EFS access points per filesystem, so at most 1000 PVs
  per StorageClass, and EFS does not enforce PVC size requests, so track usage in
  CloudWatch. ArgoCD has no health assessment for `DataScienceCluster` or
  `InferenceService`, so those Applications report Progressing indefinitely.
- **RBAC.** These charts create scoped ClusterRoles for ArgoCD rather than relying on the
  `cluster-admin` binding in [../README.md](../README.md). On the reference cluster the
  application controller is not cluster-admin. It can already create namespaces,
  Subscriptions, PersistentVolumeClaims and ClusterRoles, and cannot create
  ServiceAccounts, `jobs.batch` or `DataScienceCluster` objects, which is what the chart
  ClusterRoles supply. Check yours with `oc auth can-i <verb> <resource>
  --as=system:serviceaccount:openshift-gitops:openshift-gitops-argocd-application-controller`.

## Uninstall

```bash
oc delete -f https://raw.githubusercontent.com/awslabs/operator-for-ai-chips-on-aws/main/deploy/argocd/applicationset-performance.yaml
oc delete secret -n openshift-gitops <infra-name>-neuron-perf
```

The EFS filesystem, mount targets, security group, IAM role and policy are not managed
by GitOps and are left in place deliberately, so an uninstall cannot destroy a warm
cache. Remove them with `aws` when you mean to.
