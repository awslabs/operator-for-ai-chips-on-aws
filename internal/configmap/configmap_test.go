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
	"os"

	mockclient "github.com/awslabs/operator-for-ai-chips-on-aws/internal/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("CreateKubeletKubeRootCAConfigMap", func() {
	var (
		ctx            context.Context
		ctrl           *gomock.Controller
		mockK8sClient  *mockclient.MockClient
		operatorNS     string
		originalEnvVar string
	)

	BeforeEach(func() {
		ctx = context.Background()
		operatorNS = "test-operator-namespace"
		originalEnvVar = os.Getenv("OPERATOR_NAMESPACE")
		_ = os.Setenv("OPERATOR_NAMESPACE", operatorNS)

		ctrl = gomock.NewController(GinkgoT())
		mockK8sClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		if originalEnvVar == "" {
			_ = os.Unsetenv("OPERATOR_NAMESPACE")
		} else {
			_ = os.Setenv("OPERATOR_NAMESPACE", originalEnvVar)
		}
	})

	kubeletServingCANN := types.NamespacedName{
		Namespace: "openshift-config-managed",
		Name:      "kubelet-serving-ca",
	}
	kubeRootCANN := func(ns string) types.NamespacedName {
		return types.NamespacedName{Namespace: ns, Name: "kube-root-ca.crt"}
	}
	unifiedCANN := func(ns string) types.NamespacedName {
		return types.NamespacedName{Namespace: ns, Name: "kube-root-kubelet-ca"}
	}

	expectKubeletGetOK := func(kubeletPEM string) {
		kubeletCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeletServingCANN.Name,
				Namespace: kubeletServingCANN.Namespace,
			},
			Data: map[string]string{"ca-bundle.crt": kubeletPEM},
		}
		mockK8sClient.EXPECT().
			Get(ctx, kubeletServingCANN, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj client.Object, _ ...client.GetOption) error {
				*obj.(*corev1.ConfigMap) = *kubeletCM
				return nil
			})
	}

	expectKubeRootGetOK := func(ns, rootPEM string) {
		kubeRootCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-root-ca.crt",
				Namespace: ns,
			},
			Data: map[string]string{"ca.crt": rootPEM},
		}
		mockK8sClient.EXPECT().
			Get(ctx, kubeRootCANN(ns), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj client.Object, _ ...client.GetOption) error {
				*obj.(*corev1.ConfigMap) = *kubeRootCM
				return nil
			})
	}

	It("succeeds and creates the unified configmap when both source configmaps and keys exist", func() {
		rootPEM := "root-ca-pem"
		kubeletPEM := "kubelet-ca-pem"
		expectKubeletGetOK(kubeletPEM)
		expectKubeRootGetOK(operatorNS, rootPEM)

		mockK8sClient.EXPECT().
			Get(ctx, unifiedCANN(operatorNS), gomock.Any()).
			Return(k8serrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, unifiedCANN(operatorNS).Name))

		mockK8sClient.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
				cm := obj.(*corev1.ConfigMap)
				Expect(cm.Namespace).To(Equal(operatorNS))
				Expect(cm.Name).To(Equal("kube-root-kubelet-ca"))
				Expect(cm.Data).To(HaveKeyWithValue("ca-bundle.crt", rootPEM+"\n"+kubeletPEM))
				return nil
			})

		Expect(CreateKubeletKubeRootCAConfigMap(ctx, mockK8sClient)).To(Succeed())
	})

	It("returns an error when the kubelet-serving-ca configmap cannot be retrieved", func() {
		mockK8sClient.EXPECT().
			Get(ctx, kubeletServingCANN, gomock.Any()).
			Return(k8serrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, kubeletServingCANN.Name))

		err := CreateKubeletKubeRootCAConfigMap(ctx, mockK8sClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("kubelet-serving-ca"))
		Expect(err.Error()).To(ContainSubstring("failed to get data from configmap"))
	})

	It("returns an error when kubelet-serving-ca exists but ca-bundle.crt is missing", func() {
		kubeletCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeletServingCANN.Name,
				Namespace: kubeletServingCANN.Namespace,
			},
			Data: map[string]string{"other-key": "data"},
		}
		mockK8sClient.EXPECT().
			Get(ctx, kubeletServingCANN, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj client.Object, _ ...client.GetOption) error {
				*obj.(*corev1.ConfigMap) = *kubeletCM
				return nil
			})

		err := CreateKubeletKubeRootCAConfigMap(ctx, mockK8sClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ca-bundle.crt"))
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("returns an error when the kube-root-ca.crt configmap cannot be retrieved", func() {
		expectKubeletGetOK("kubelet-pem")

		mockK8sClient.EXPECT().
			Get(ctx, kubeRootCANN(operatorNS), gomock.Any()).
			Return(k8serrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, kubeRootCANN(operatorNS).Name))

		err := CreateKubeletKubeRootCAConfigMap(ctx, mockK8sClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("kube-root-ca.crt"))
		Expect(err.Error()).To(ContainSubstring("failed to get data from configmap"))
	})

	It("returns an error when kube-root-ca.crt exists but ca.crt is missing", func() {
		expectKubeletGetOK("kubelet-pem")

		kubeRootCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-root-ca.crt",
				Namespace: operatorNS,
			},
			Data: map[string]string{"wrong-key": "data"},
		}
		mockK8sClient.EXPECT().
			Get(ctx, kubeRootCANN(operatorNS), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj client.Object, _ ...client.GetOption) error {
				*obj.(*corev1.ConfigMap) = *kubeRootCM
				return nil
			})

		err := CreateKubeletKubeRootCAConfigMap(ctx, mockK8sClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ca.crt"))
		Expect(err.Error()).To(ContainSubstring("not found"))
	})
})
