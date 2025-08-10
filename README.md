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


### Repository layout essentials

- CRD: `config/crd/bases/k8s.aws_deviceconfigs.yaml` defines `k8s.aws/v1alpha1`, `DeviceConfig`
- Controller: `internal/controllers/device_config_reconciler.go`
- KMM integration: `internal/kmmmodule/kmmmodule.go` (module name `neuron`)
- Custom scheduler: `internal/customscheduler/customscheduler.go`
- Node metrics: `internal/nodemetrics/nodemetrics.go`
- NFD rule: `config/nfd/nfd-rule.yaml` (labels Neuron PCI devices)
- Deploy overlay: `config/default` (includes CRDs, RBAC, manager, NFD rule)


### Build the manager image

By default the image is `ghcr.io/awslabs/operator-for-ai-chips-on-aws/operator:latest`. Override `IMG` to your registry.

```bash
# From repository root
make manager

# Build and push to your registry
make docker-build IMG=<your-registry>/<repo>/aws-neuron-operator:latest
make docker-push IMG=<your-registry>/<repo>/aws-neuron-operator:latest
```


### Deploy on OpenShift (direct deploy)

Required: Install the Kernel Module Management (KMM) and Node Feature Discovery (NFD) operators first.

This deploys the operator controller, CRDs, RBAC, and the NFD rule into namespace `ai-operator-on-aws`.

```bash
# Ensure NFD and KMM operators are installed first

# Deploy the operator
make deploy IMG=<your-registry>/<repo>/aws-neuron-operator:latest

# Verify controller is running
oc get pods -n ai-operator-on-aws
```

If you prefer to apply only the NFD rule manually (for troubleshooting), you can:

```bash
oc apply -f config/nfd/nfd-rule.yaml
```

This rule adds labels to nodes with Neuron PCI devices (vendor `1d0f`, specific device IDs) and can be used for scheduling.


### Install via OLM (optional)

You can publish an Operator bundle and index image, then create a CatalogSource.

Required: Install the Kernel Module Management (KMM) and Node Feature Discovery (NFD) operators before installing the AWS Neuron operator via OLM. When installing via OLM, the NFD NodeFeatureRule is not part of the bundle and must be applied manually; use the rule below.

```bash
# Generate bundle from manifests
make bundle PROJECT_VERSION=0.0.1 \
  IMAGE_TAG_BASE=<your-registry>/<repo>/aws-neuron-operator \
  DEFAULT_CHANNEL=stable CHANNELS=stable

# Build and push bundle image
make bundle-build BUNDLE_IMG=<your-registry>/<repo>/aws-neuron-operator-bundle:v0.0.1
podman push <your-registry>/<repo>/aws-neuron-operator-bundle:v0.0.1 || docker push <...>

# Build and push index image
make index BUNDLE_IMG=<your-registry>/<repo>/aws-neuron-operator-bundle:v0.0.1 \
  IMAGE_TAG_BASE=<your-registry>/<repo>/aws-neuron-operator PROJECT_VERSION=0.0.1
podman push <your-registry>/<repo>/aws-neuron-operator-index:v0.0.1 || docker push <...>
```

Create a CatalogSource in `openshift-marketplace` referencing your index image, then create a Subscription in your target namespace.

Example CatalogSource:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: aws-neuron-operator-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: <your-registry>/<repo>/aws-neuron-operator-index:v0.0.1
  displayName: AWS Neuron Operator Catalog
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
- Optional: `imageRepoSecret`, `selector` (defaults to `feature.node.kubernetes.io/pci-1d0f.present: "true"`)

Example:

```yaml
apiVersion: k8s.aws/v1alpha1
kind: DeviceConfig
metadata:
  name: neuron
  namespace: ai-operator-on-aws
spec:
  driversImage: <registry>/<repo>/neuron-drivers  # actual pull at runtime will use <image>-$KERNEL_VERSION
  devicePluginImage: <registry>/<repo>/neuron-device-plugin:latest
  customSchedulerImage: <registry>/<repo>/kube-scheduler-neuron:latest
  schedulerExtensionImage: <registry>/<repo>/neuron-scheduler-extension:latest
  selector:
    feature.node.kubernetes.io/pci-1d0f.present: "true"
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
oc describe nodes | egrep "feature.node.kubernetes.io/pci-1d0f.present|aws-neuron"
```

- Resources exposed by device plugin:
```bash
oc describe node | egrep "Resource.*Requests|aws.amazon.com/neuron|aws.amazon.com/neuroncore"
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

