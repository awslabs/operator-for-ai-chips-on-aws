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

package kmmmodule

import (
	_ "embed"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	awslabsv1beta1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1beta1"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/constants"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
)

const (
	gpuDriverModuleName = "neuron"
)

//go:generate mockgen -source=kmmmodule.go -package=kmmmodule -destination=mock_kmmmodule.go KMMModuleAPI
type KMMModuleAPI interface {
	SetKMMModuleAsDesired(mod *kmmv1beta1.Module, devConfig *awslabsv1beta1.DeviceConfig) error
}

type kmmModule struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewKMMModule(client client.Client, scheme *runtime.Scheme) KMMModuleAPI {
	return &kmmModule{
		client: client,
		scheme: scheme,
	}
}

func (km *kmmModule) SetKMMModuleAsDesired(mod *kmmv1beta1.Module, devConfig *awslabsv1beta1.DeviceConfig) error {
	err := setKMMModuleLoader(mod, devConfig)
	if err != nil {
		return fmt.Errorf("failed to set KMM Module: %v", err)
	}
	setKMMDevicePlugin(mod, devConfig)
	return controllerutil.SetControllerReference(devConfig, mod, km.scheme)
}

func setKMMModuleLoader(mod *kmmv1beta1.Module, devConfig *awslabsv1beta1.DeviceConfig) error {
	driversImage := devConfig.Spec.DriversImage + "-$KERNEL_VERSION"

	mod.Spec.ModuleLoader = &kmmv1beta1.ModuleLoaderSpec{
		Container: kmmv1beta1.ModuleLoaderContainerSpec{
			Modprobe: kmmv1beta1.ModprobeSpec{
				ModuleName: gpuDriverModuleName,
			},
			KernelMappings: []kmmv1beta1.KernelMapping{
				{
					Regexp:                "^.+$",
					ContainerImage:        driversImage,
					InTreeModulesToRemove: []string{gpuDriverModuleName},
				},
			},
			ImagePullPolicy: v1.PullAlways,
			Version:         devConfig.Spec.DriverVersion,
		},
	}
	mod.Spec.ModuleLoader.ServiceAccountName = "awslabs-gpu-operator-kmm-module-loader"
	mod.Spec.ImageRepoSecret = devConfig.Spec.ImageRepoSecret
	mod.Spec.Selector = getNodeSelector(devConfig)
	mod.Spec.Tolerations = []v1.Toleration{
		{
			Key:      constants.UpgradeTaintTolerationKey,
			Value:    "true",
			Operator: v1.TolerationOpEqual,
			Effect:   v1.TaintEffectNoExecute,
		},
	}
	return nil
}

func setKMMDevicePlugin(mod *kmmv1beta1.Module, devConfig *awslabsv1beta1.DeviceConfig) {
	devicePluginImage := devConfig.Spec.DevicePluginImage
	hostPathDirectory := v1.HostPathDirectory
	mod.Spec.DevicePlugin = &kmmv1beta1.DevicePluginSpec{
		ServiceAccountName: "awslabs-gpu-operator-kmm-device-plugin",
		Container: kmmv1beta1.DevicePluginContainerSpec{
			Image: devicePluginImage,
			Env: []v1.EnvVar{
				{
					Name: "NODE_NAME",
					ValueFrom: &v1.EnvVarSource{
						FieldRef: &v1.ObjectFieldSelector{
							FieldPath: "spec.nodeName",
						},
					},
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "sys",
					MountPath: "/sys",
				},
			},
		},
		Volumes: []v1.Volume{
			{
				Name: "sys",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/sys",
						Type: &hostPathDirectory,
					},
				},
			},
		},
	}
}

func getNodeSelector(devConfig *awslabsv1beta1.DeviceConfig) map[string]string {
	if devConfig.Spec.Selector != nil {
		return devConfig.Spec.Selector
	}

	ns := make(map[string]string, 0)
	ns[fmt.Sprintf("feature.node.kubernetes.io/pci-%s.present", awslabsv1beta1.PCIVendorID)] = "true"
	return ns
}
