/*
Copyright 2026.

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

package dradriver

import (
	"fmt"

	awslabsv1beta1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1beta1"
	"github.com/rh-ecosystem-edge/kernel-module-management/pkg/labels"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	draDriverServiceAccount = "awslabs-gpu-operator-dra-driver"
	healthCheckPort         = 51515
)

//go:generate mockgen -source=dradriver.go -package=dradriver -destination=mock_dradriver.go DRADriver
type DRADriver interface {
	SetDRADriverAsDesired(ds *appsv1.DaemonSet, devConfig *awslabsv1beta1.DeviceConfig) error
}

type draDriver struct {
	scheme *runtime.Scheme
}

func NewDRADriver(scheme *runtime.Scheme) DRADriver {
	return &draDriver{
		scheme: scheme,
	}
}

func (d *draDriver) SetDRADriverAsDesired(ds *appsv1.DaemonSet, devConfig *awslabsv1beta1.DeviceConfig) error {
	if ds == nil {
		return fmt.Errorf("daemon set is not initialized, zero pointer")
	}

	matchLabels := map[string]string{
		"app.kubernetes.io/name":      "neuron-dra-driver",
		"app.kubernetes.io/component": "aws-neuron",
		"app.kubernetes.io/part-of":   "aws-neuron",
	}

	nodeSelector := map[string]string{
		labels.GetKernelModuleReadyNodeLabel(devConfig.Namespace, devConfig.Name): "",
	}

	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: v1.PodSpec{
				ServiceAccountName: draDriverServiceAccount,
				HostNetwork:        true,
				NodeSelector:       nodeSelector,
				Containers: []v1.Container{
					{
						Name:            "neuron-dra-driver",
						Image:           devConfig.Spec.DRADriverImage,
						Command:         []string{"k8s-neuron-dra-driver"},
						ImagePullPolicy: v1.PullIfNotPresent,
						Env:             getEnvVars(),
						Resources:       getResources(),
						VolumeMounts:    getVolumeMounts(),
						LivenessProbe:   getLivenessProbe(),
					},
				},
				Volumes: getVolumes(),
			},
		},
	}

	return controllerutil.SetControllerReference(devConfig, ds, d.scheme)
}

func getEnvVars() []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name: "NODE_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		{
			Name: "POD_UID",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.uid",
				},
			},
		},
		{
			Name:  "CDI_ROOT",
			Value: "/var/run/cdi",
		},
		{
			Name:  "KUBELET_REGISTRAR_DIRECTORY_PATH",
			Value: "/var/lib/kubelet/plugins_registry",
		},
		{
			Name:  "KUBELET_PLUGINS_DIRECTORY_PATH",
			Value: "/var/lib/kubelet/plugins",
		},
		{
			Name:  "HEALTHCHECK_PORT",
			Value: "51515",
		},
	}
}

func getResources() v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("10m"),
			v1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("20m"),
			v1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
}

func getVolumeMounts() []v1.VolumeMount {
	return []v1.VolumeMount{
		{
			Name:      "kubelet-plugins-dir",
			MountPath: "/var/lib/kubelet/plugins",
		},
		{
			Name:      "kubelet-registry-dir",
			MountPath: "/var/lib/kubelet/plugins_registry",
		},
		{
			Name:      "cdi-dir",
			MountPath: "/var/run/cdi",
		},
	}
}

func getVolumes() []v1.Volume {
	return []v1.Volume{
		{
			Name: "kubelet-plugins-dir",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/kubelet/plugins",
				},
			},
		},
		{
			Name: "kubelet-registry-dir",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/lib/kubelet/plugins_registry",
				},
			},
		},
		{
			Name: "cdi-dir",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/run/cdi",
				},
			},
		},
	}
}

func getLivenessProbe() *v1.Probe {
	return &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			GRPC: &v1.GRPCAction{
				Port:    healthCheckPort,
				Service: ptr.To[string]("liveness"),
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}
