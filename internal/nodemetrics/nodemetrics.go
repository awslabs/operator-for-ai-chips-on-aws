/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodemetrics

import (
	"fmt"

	awslabsv1alpha1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1alpha1"
	"github.com/rh-ecosystem-edge/kernel-module-management/pkg/labels"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	metricsPortName       = "metrics"
	metricsPort           = 8000
	metricsServiceAccount = "awslabs-gpu-operator-node-metrics"
	metricsImage          = "public.ecr.aws/neuron/neuron-monitor:1.3.0"
)

//go:generate mockgen -source=nodemetrics.go -package=nodemetrics -destination=mock_nodemetrics.go NodeMetrics
type NodeMetrics interface {
	SetNodeMetricsAsDesired(ds *appsv1.DaemonSet, devConfig *awslabsv1alpha1.DeviceConfig) error
}

type nodeMetrics struct {
	scheme *runtime.Scheme
}

func NewNodeMetrcis(scheme *runtime.Scheme) NodeMetrics {
	return &nodeMetrics{
		scheme: scheme,
	}
}

func (nm *nodeMetrics) SetNodeMetricsAsDesired(ds *appsv1.DaemonSet, devConfig *awslabsv1alpha1.DeviceConfig) error {
	if ds == nil {
		return fmt.Errorf("daemon set is not initialized, zero pointer")
	}

	volumes, volumesMounts := getVolumesAndMount()
	ports := getPorts()

	matchLabels := map[string]string{
		"app.kubernetes.io/component": "aws-neuron",
		"app.kubernetes.io/name":      "aws-neuron",
		"app.kubernetes.io/part-of":   "aws-neuron",
		"app.kubernetes.io/role":      "aws-neuron-gpu-metrics",
	}
	nodeSelector := map[string]string{labels.GetKernelModuleReadyNodeLabel(devConfig.Namespace, devConfig.Name): ""}
	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "node-metrics-container",
						Image: metricsImage,
						/*
							Args: []string{
								"--port",
								"8000",
								"--neuron-monitor-config",
								"/opt/aws/neuron/bin/neuron-monitor.conf",
							},
						*/
						Command: []string{
							"/bin/bash",
							"-c",
						},
						Args: []string{`
							trap "kill -TERM 0; exit 0" TERM INT; \
							/opt/bin/entrypoint.sh \
							--port 8000 \
							--neuron-monitor-config /opt/aws/neuron/bin/neuron-monitor.conf & \
							wait`,
						},
						/*
							Command:         []string{
								"/bin/bash",
								"-c",
								"|",

								"/opt/bin/entrypoint.sh"
							},
						*/
						ImagePullPolicy: v1.PullAlways,
						SecurityContext: &v1.SecurityContext{
							Privileged: ptr.To[bool](true),
							RunAsUser:  ptr.To[int64](0),
						},
						VolumeMounts: volumesMounts,
						Ports:        ports,
					},
				},
				NodeSelector:       nodeSelector,
				ServiceAccountName: metricsServiceAccount,
				Volumes:            volumes,
			},
		},
	}

	return controllerutil.SetControllerReference(devConfig, ds, nm.scheme)
}

func getVolumesAndMount() ([]v1.Volume, []v1.VolumeMount) {
	containerVolumeMounts := []v1.VolumeMount{
		{
			Name:      "root-volume",
			MountPath: "/host/root",
		},
		{
			Name:      "sys-volume",
			MountPath: "/host/sys",
		},
		{
			Name:      "config-volume",
			MountPath: "/opt/aws/neuron/bin/neuron-monitor.conf",
			SubPath:   "neuron-monitor.conf",
			ReadOnly:  true,
		},
	}

	hostPathDirectory := v1.HostPathDirectory
	volumes := []v1.Volume{
		{
			Name: "root-volume",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "sys-volume",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/sys",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "config-volume",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "awslabs-gpu-operator-node-metrics-configmap",
					},
				},
			},
		},
	}

	return volumes, containerVolumeMounts
}

func getPorts() []v1.ContainerPort {
	return []v1.ContainerPort{
		{
			Name:          metricsPortName,
			HostPort:      metricsPort,
			ContainerPort: metricsPort,
			Protocol:      v1.ProtocolTCP,
		},
	}
}
