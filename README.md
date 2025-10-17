## AWS Neuron GPU Operator for OpenShift

The AWS Neuron GPU Operator automates enabling AWS Neuron devices on OpenShift clusters. It:

- Deploys kernel modules for Neuron devices via Kernel Module Management (KMM)
- Deploys the Neuron device plugin to advertise resources to the scheduler
- Deploys a custom Neuron-aware scheduler and a scheduler extension
- Deploys a node-metrics DaemonSet for basic telemetry

It reconciles a custom resource `DeviceConfig` to configure images and targeting of nodes.

### Prerequisites

- OpenShift 4.19 or newer
- `oc` CLI and cluster-admin permissions
- Cluster has AWS Neuron-capable hardware (PCI vendor ID 1d0f)
- The following operators installed from OperatorHub:
  - Node Feature Discovery (NFD)
  - Kernel Module Management (KMM)
- An image registry accessible to the cluster (internal OpenShift registry or external registry)
- The build process uses "docker buildx". In case you are compiling on the non-amd64 platform, the multiarch should be enabled:
  `docker buildx create --use --name multiarch`
  `docker buildx inspect --bootstrap`


### Repository layout essentials

- CRD: `config/crd/bases/k8s.aws_deviceconfigs.yaml` defines `k8s.aws/v1alpha1`, `DeviceConfig`
- Controller: `internal/controllers/device_config_reconciler.go`
- KMM integration: `internal/kmmmodule/kmmmodule.go` (module name `neuron`)
- Custom scheduler: `internal/customscheduler/customscheduler.go`
- Node metrics: `internal/nodemetrics/nodemetrics.go`
- NFD rule: `config/nfd/nfd-rule.yaml` (labels Neuron PCI devices)
- Deploy overlay: `config/default` (includes CRDs, RBAC, manager, NFD rule)


### Build the manager image
Currently we are supporting building the image for linux amd64 architecture only.
By default the image is `ghcr.io/awslabs/operator-for-ai-chips-on-aws/operator:latest`. Override `IMG` to your registry.

```bash
# From repository root (just for quick verification of the compilation)
make manager

# Build and push to your registry
make docker-build IMG=<your-registry>/<repo>/aws-neuron-operator:<tag>
```


### Deploy on OpenShift (direct deploy)

Required: Install the Kernel Module Management (KMM) and Node Feature Discovery (NFD) operators first.
Important: once the NFD operator is installed, the NFD CR needs to be applied to the cluster. The default NFD CR is supplied by the OLM UI,
and can be applied as is.

This deploys the operator controller, CRDs, RBAC, and the NFD rule into namespace `ai-operator-on-aws`.

```bash
# Ensure NFD and KMM operators are installed first

# Deploy the operator
make deploy IMG=<your-registry>/<repo>/aws-neuron-operator:latest

# Verify controller is running
oc get pods -n ai-operator-on-aws
```

This rule adds labels to nodes with Neuron PCI devices (vendor `1d0f`, specific device IDs) and can be used for scheduling.


### Install via OLM (optional)

In order to deploy operator via OLM, you need to:
1. create a bundle image and push it to the image registry
2. create an index image, that will point to the bundle image
3. apply Namespace, OperatorGroup, CatalogSource(that will point to the index image) and Subscription to the cluster

Note: this is mainly a debug option, in case you want to debug using your images. In case you are creating index and bundle images, you need to make sure that the appropriate repos are created first. By default, the images are pointing to the Redhat repos, which are currently used to contain the official images, and the tag for the images is created from the value of PROJECT_VERSION

Required: Install the Kernel Module Management (KMM) and Node Feature Discovery (NFD) operators before installing the AWS Neuron operator via OLM. When installing via OLM, the NFD NodeFeatureRule is not part of the bundle and must be applied manually; use the rule below.

```bash
# Generate bundle from manifests
make bundle PROJECT_VERSION=<project version> \
  IMAGE_TAG_BASE=<your-registry>/<repo>/aws-neuron-operator:<tag> \
  DEFAULT_CHANNEL=stable CHANNELS=stable

# Build and push bundle image
make bundle-build BUNDLE_IMG=<your-registry>/<repo>/aws-neuron-operator-bundle:<tag>

# Build and push index image
make index BUNDLE_IMG=<your-registry>/<repo>/aws-neuron-operator-bundle:<tag> \
  INDEX_IMG=<your-registry>/<repo>/aws-neuron-operator-index:<tag>
```

Create the Namespace.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    control-plane: controller-manager
    security.openshift.io/scc.podSecurityLabelSync: 'true'
  name: ai-operator-on-aws
```

Create the OperatorGroup.

```yaml
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: aws-neuron-operator
  namespace: ai-operator-on-aws
```

Create a CatalogSource in `openshift-marketplace` referencing your index image, then create a Subscription in your target namespace.

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: aws-neuron-operator
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: <your-registry>/<repo>/aws-neuron-operator-index:<tag>
  displayName: AWS Neuron Operator Catalog
```

Create the Subscription.

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: aws-neuron-operator-sub
  namespace: ai-operator-on-aws
spec:
  channel: "alpha"
  installPlanApproval: Automatic
  name: aws-neuron-operator
  source: aws-neuron-operator
  sourceNamespace: openshift-marketplace
```

Apply the NFD NodeFeatureRule (required for OLM installs):

```yaml
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
                device: {op: In, value: [
                  "7064",
                  "7065",
                  "7066",
                  "7067",
                  "7164",
                  "7264",
                  "7364",
                ]}
```

Apply it:

```bash
echo "<paste YAML above>" | oc apply -f -
```


### Configure the operator with DeviceConfig

The operator reconciles `DeviceConfig` objects to install kernel modules, deploy the device plugin, and run the Neuron-aware scheduler and extension.

Required spec fields (see CRD for details):
- `driversImage`: Image containing the Neuron kernel module artifacts; KMM appends `-$KERNEL_VERSION` at runtime
- `devicePluginImage`: Neuron device plugin image
- `customSchedulerImage`: Kube scheduler image extended for Neuron
- `schedulerExtensionImage`: Neuron scheduler extension image

Optional fields: 
- `imageRepoSecret`: the secret that contains tokens for pulling the images
- `selector`: defines which nodes to target for driver/device-plugin deployment

Example:

```yaml
apiVersion: k8s.aws/v1alpha1
kind: DeviceConfig
metadata:
  name: neuron
  namespace: ai-operator-on-aws
spec:
  driversImage: ghcr.io/awslabs/kmod-with-kmm-for-ai-chips-on-aws/neuron-driver:2.22.2.0  # actual pull at runtime will use <image>-$KERNEL_VERSION
  devicePluginImage: public.ecr.aws/neuron/neuron-device-plugin:2.23.30.0
  customSchedulerImage: public.ecr.aws/eks-distro/kubernetes/kube-scheduler:v1.28.5-eks-1-28-latest
  schedulerExtensionImage: public.ecr.aws/neuron/neuron-scheduler:2.23.30.0
  imageRepoSecret:
    name: image-repo-secret
  selector:
    feature.node.kubernetes.io/aws-neuron: "true"
```

Apply it:

```bash
oc apply -f deviceconfig.yaml
```


### Verify

- Operator pods:
```bash
oc get pods -n ai-operator-on-aws
```

- KMM Module and DaemonSets:
```bash
oc get modules.kmm.sigs.x-k8s.io -A
oc get ds -n ai-operator-on-aws
```

- Node labels from NFD:
```bash
oc describe nodes | egrep "feature.node.kubernetes.io/aws-neuron"
```

- Resources exposed by device plugin (per node):
```bash
kubectl get nodes -o json | jq -r '
  .items[]
  | select(((.status.capacity["aws.amazon.com/neuron"] // "0") | tonumber) > 0)
  | .metadata.name as $name
  | "\($name)\n  aws.amazon.com/neuron:      \(.status.capacity["aws.amazon.com/neuron"])\n  aws.amazon.com/neuroncore:  \(.status.capacity["aws.amazon.com/neuroncore"])"
'
```

- Schedule a test pod requesting Neuron resources (image must support Neuron runtime):
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: neuron-test
spec:
  containers:
  - name: app
    image: <neuron-enabled-image>
    resources:
      limits:
        aws.amazon.com/neuron: 1
      requests:
        aws.amazon.com/neuron: 1
  restartPolicy: Never
```

### Development

Useful targets:
- `make manager` — build controller binary
- `make docker-build IMG=...` and `make docker-push IMG=...`
- `make deploy IMG=...` — deploy controller, CRDs, RBAC, and NFD rule
- `make uninstall` — remove CRDs
- `make undeploy` — remove controller/RBAC
- `make bundle` / `make bundle-build` / `make index` — OLM artifacts


### License

This project is licensed under the Apache-2.0 License. See `LICENSE`.

