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

package configmap

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	kubeletCAConfigMap             = "kubelet-serving-ca"
	kubeletCANamespace             = "openshift-config-managed"
	kubeletCAKey                   = "ca-bundle.crt"
	kubeRootCAConfigMap            = "kube-root-ca.crt"
	kubeRootCAKey                  = "ca.crt"
	kubeletKubeRootCAConfigMapName = "kube-root-kubelet-ca"
	kubeletKubeRootCAKey           = "ca-bundle.crt"
)

// CreateKubeletKubeRootCAConfigMap creates a CA configmap that
// contain both kube-root CAs and kubelet CAs.
func CreateKubeletKubeRootCAConfigMap(ctx context.Context, client client.Client) error {
	logger := log.FromContext(ctx).WithName("configmap-unification")

	// Get the operator namespace from environment variable
	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		return fmt.Errorf("OPERATOR_NAMESPACE environment variable is not set")
	}

	logger.Info("start CreateKubeletKubeRootCAConfigMap", "namespace", operatorNamespace)

	kubeletCMData, err := getConfigMapData(ctx, client, kubeletCANamespace, kubeletCAConfigMap, kubeletCAKey)
	if err != nil {
		return fmt.Errorf("failed to get data from configmap %s/%s: %v", kubeletCANamespace, kubeletCAConfigMap, err)
	}

	kubeRootCMData, err := getConfigMapData(ctx, client, operatorNamespace, kubeRootCAConfigMap, kubeRootCAKey)
	if err != nil {
		return fmt.Errorf("failed to get data from configmap %s/%s: %v", operatorNamespace, kubeRootCAConfigMap, err)
	}

	unifiedCAContent := kubeRootCMData + "\n" + kubeletCMData

	kubeletRootCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: operatorNamespace,
			Name:      kubeletKubeRootCAConfigMapName,
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, client, kubeletRootCM, func() error {
		if kubeletRootCM.Data == nil {
			kubeletRootCM.Data = make(map[string]string)
		}

		kubeletRootCM.Data[kubeletKubeRootCAKey] = unifiedCAContent
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create or update configmap %s/%s: %v", operatorNamespace, kubeletKubeRootCAConfigMapName, err)
	}

	logger.Info("create kubelet-root-ca configmap succesfully", "opResult", opResult)
	return nil
}

func getConfigMapData(ctx context.Context, client client.Client, cmNamespace, cmName, cmKey string) (string, error) {
	cm := &corev1.ConfigMap{}
	nsn := types.NamespacedName{
		Namespace: cmNamespace,
		Name:      cmName,
	}
	err := client.Get(ctx, nsn, cm)
	if err != nil {
		return "", fmt.Errorf("failed to get configmap %s/%s: %v", cmNamespace, cmName, err)
	}

	data, exists := cm.Data[cmKey]
	if !exists {
		return "", fmt.Errorf("key %s not found in configmap %s/%s", cmKey, cmNamespace, cmName)
	}

	return data, nil
}
