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

package customscheduler

import (
	awslabsv1beta1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

//go:generate mockgen -source=customscheduler.go -package=customscheduler -destination=mock_customscheduler.go CustomScheduler
type CustomScheduler interface {
	SetCustomSchedulerAsDesired(dp *appsv1.Deployment, devConfig *awslabsv1beta1.DeviceConfig)
	SetCustomSchedulerExtensionAsDesired(dp *appsv1.Deployment, devConfig *awslabsv1beta1.DeviceConfig)
}

type customScheduler struct {
	scheme *runtime.Scheme
}

func NewCustomScheduler(scheme *runtime.Scheme) CustomScheduler {
	return &customScheduler{
		scheme: scheme,
	}
}

func (cs *customScheduler) SetCustomSchedulerAsDesired(dp *appsv1.Deployment, devConfig *awslabsv1beta1.DeviceConfig) {
	containerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "config-volume",
			MountPath: "/etc/kubernetes/neuron-scheduler",
		},
	}
	volumes := []corev1.Volume{
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "awslabs-gpu-operator-neuron-scheduler-config",
					},
				},
			},
		},
	}

	matchLabels := map[string]string{
		"app.kubernetes.io/name":      "aws-neuron",
		"app.kubernetes.io/component": "aws-neuron",
		"app.kubernetes.io/part-of":   "aws-neuron",
		"app.kubernetes.io/instance":  "neuron-release",
		"tier":                        "control-plane",
	}

	dp.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
		Replicas: ptr.To[int32](1),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "awslabs-gpu-operator-neuron-scheduler",
				Containers: []corev1.Container{
					{
						Name:  "kube-second-scheduler",
						Image: devConfig.Spec.CustomSchedulerImage,
						Args: []string{
							"--config=/etc/kubernetes/neuron-scheduler/neuron_scheduler_config.yaml",
							"--leader-elect=true",
							"--v=2",
						},
						Command:         []string{"/usr/local/bin/kube-scheduler"},
						ImagePullPolicy: corev1.PullIfNotPresent,
						LivenessProbe: &corev1.Probe{
							InitialDelaySeconds: 15,
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Port:   intstr.FromInt(10259),
									Scheme: corev1.URISchemeHTTPS,
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Port:   intstr.FromInt(10259),
									Scheme: corev1.URISchemeHTTPS,
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("0.1"),
							},
						},
						SecurityContext: &corev1.SecurityContext{Privileged: ptr.To[bool](false)},
						VolumeMounts:    containerVolumeMounts,
					},
				},
				Volumes: volumes,
			},
		},
	}
}

func (cs *customScheduler) SetCustomSchedulerExtensionAsDesired(dp *appsv1.Deployment, devConfig *awslabsv1beta1.DeviceConfig) {
	matchLabels := map[string]string{
		"app.kubernetes.io/name":      "aws-neuron",
		"app.kubernetes.io/instance":  "neuron-scheduler-extension",
		"app.kubernetes.io/component": "aws-neuron",
		"app.kubernetes.io/part-of":   "aws-neuron",
	}
	dp.Spec = appsv1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: matchLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: matchLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "awslabs-gpu-operator-neuron-scheduler",
				PriorityClassName:  "system-node-critical",
				SchedulerName:      "neuron-scheduler",
				Containers: []corev1.Container{
					{
						Name:            "neuron-scheduler-extension",
						Image:           devConfig.Spec.SchedulerExtensionImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							{
								Name:  "PORT",
								Value: "12345",
							},
						},
					},
				},
			},
		},
	}
}
