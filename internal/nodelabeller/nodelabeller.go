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

package nodelabeller

import (
	"fmt"

	"github.com/rh-ecosystem-edge/kernel-module-management/pkg/labels"
	awslabsv1alpha1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//go:generate mockgen -source=nodelabeller.go -package=nodelabeller -destination=mock_nodelabeller.go NodeLabeller
type NodeLabeller interface {
	SetNodeLabellerAsDesired(ds *appsv1.DaemonSet, devConfig *awslabsv1alpha1.DeviceConfig) error
}

type nodeLabeller struct {
	scheme *runtime.Scheme
}

func NewNodeLabeller(scheme *runtime.Scheme) NodeLabeller {
	return &nodeLabeller{
		scheme: scheme,
	}
}

func (nl *nodeLabeller) SetNodeLabellerAsDesired(ds *appsv1.DaemonSet, devConfig *awslabsv1alpha1.DeviceConfig) error {
	if ds == nil {
		return fmt.Errorf("daemon set is not initialized, zero pointer")
	}
	containerVolumeMounts := []v1.VolumeMount{
		{
			Name:      "dev-volume",
			MountPath: "/dev",
		},
		{
			Name:      "sys-volume",
			MountPath: "/sys",
		},
	}

	hostPathDirectory := v1.HostPathDirectory

	volumes := []v1.Volume{
		{
			Name: "dev-volume",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/dev",
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
	}

	matchLabels := map[string]string{"daemonset-name": devConfig.Name}
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
						Name:            "node-labeller-container",
						Image:           "",
						ImagePullPolicy: v1.PullAlways,
						SecurityContext: &v1.SecurityContext{Privileged: pointer.Bool(true)},
						VolumeMounts:    containerVolumeMounts,
					},
				},
				PriorityClassName:  "system-node-critical",
				NodeSelector:       nodeSelector,
				ServiceAccountName: "awslabs-gpu-operator-node-labeller",
				Volumes:            volumes,
			},
		},
	}

	return controllerutil.SetControllerReference(devConfig, ds, nl.scheme)
}
