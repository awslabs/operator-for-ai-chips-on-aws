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

package controllers

import (
	"context"
	"fmt"

	awslabsv1alpha1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1alpha1"
	mock_client "github.com/awslabs/operator-for-ai-chips-on-aws/internal/client"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/customscheduler"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/kmmmodule"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/nodemetrics"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/upgrade"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	devConfigName      = "devConfigName"
	devConfigNamespace = "devConfigNamespace"
)

var _ = Describe("Reconcile", func() {
	var (
		mockHelper *MockdeviceConfigReconcilerHelperAPI
		dcr        *DeviceConfigReconciler
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		mockHelper = NewMockdeviceConfigReconcilerHelperAPI(ctrl)
		dcr = &DeviceConfigReconciler{
			helper: mockHelper,
		}
	})

	ctx := context.Background()

	DescribeTable("reconciler error flow", func(setFinalizerError,
		handleKMMModuleError,
		handleModuleVersionUpgradeError,
		handleCustomSchedulerError,
		handleMetricsError bool) {
		devConfig := &awslabsv1alpha1.DeviceConfig{}
		if setFinalizerError {
			mockHelper.EXPECT().setFinalizer(ctx, devConfig).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockHelper.EXPECT().setFinalizer(ctx, devConfig).Return(nil)
		if handleKMMModuleError {
			mockHelper.EXPECT().handleKMMModule(ctx, devConfig).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockHelper.EXPECT().handleKMMModule(ctx, devConfig).Return(nil)
		if handleModuleVersionUpgradeError {
			mockHelper.EXPECT().handleModuleVersionUpgrade(ctx, devConfig).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockHelper.EXPECT().handleModuleVersionUpgrade(ctx, devConfig).Return(nil)
		if handleCustomSchedulerError {
			mockHelper.EXPECT().handleCustomScheduler(ctx, devConfig).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockHelper.EXPECT().handleCustomScheduler(ctx, devConfig).Return(nil)
		if handleMetricsError {
			mockHelper.EXPECT().handleNodeMetrics(ctx, devConfig).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		mockHelper.EXPECT().handleNodeMetrics(ctx, devConfig).Return(nil)

	executeTestFunction:

		res, err := dcr.Reconcile(ctx, devConfig)
		if setFinalizerError || handleKMMModuleError || handleModuleVersionUpgradeError || handleCustomSchedulerError || handleMetricsError {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
		}
	},
		Entry("good flow, no requeue", false, false, false, false, false),
		Entry("setFinalizer failed", true, false, false, false, false),
		Entry("handleKMMModule failed", false, true, false, false, false),
		Entry("handleModuleVersionUpgrade failed", false, false, true, false, false),
		Entry("handleCustomScheduler failed", false, false, false, true, false),
		Entry("handleMetrics failed", false, false, false, false, true),
	)

	It("device config finalization", func() {
		devConfig := &awslabsv1alpha1.DeviceConfig{}
		devConfig.SetDeletionTimestamp(&metav1.Time{})

		mockHelper.EXPECT().finalizeDeviceConfig(ctx, devConfig).Return(nil)

		res, err := dcr.Reconcile(ctx, devConfig)

		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))

		mockHelper.EXPECT().finalizeDeviceConfig(ctx, devConfig).Return(fmt.Errorf("some error"))

		res, err = dcr.Reconcile(ctx, devConfig)
		Expect(err).To(HaveOccurred())
		Expect(res).To(Equal(ctrl.Result{}))
	})
})

var _ = Describe("setFinalizer", func() {
	var (
		kubeClient *mock_client.MockClient
		dcrh       deviceConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		dcrh = newDeviceConfigReconcilerHelper(kubeClient, nil, nil, nil, nil, nil)
	})

	ctx := context.Background()

	It("good flow", func() {
		devConfig := &awslabsv1alpha1.DeviceConfig{}

		kubeClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(nil)

		err := dcrh.setFinalizer(ctx, devConfig)
		Expect(err).ToNot(HaveOccurred())

		err = dcrh.setFinalizer(ctx, devConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	It("error flow", func() {
		devConfig := &awslabsv1alpha1.DeviceConfig{}

		kubeClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))

		err := dcrh.setFinalizer(ctx, devConfig)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("finalizeDeviceConfig", func() {
	var (
		kubeClient    *mock_client.MockClient
		upgradeHelper *upgrade.MockUpgradeAPI
		dcrh          deviceConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		upgradeHelper = upgrade.NewMockUpgradeAPI(ctrl)
		dcrh = newDeviceConfigReconcilerHelper(kubeClient, nil, upgradeHelper, nil, nil, nil)
	})

	ctx := context.Background()
	devConfig := &awslabsv1alpha1.DeviceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devConfigName,
			Namespace: devConfigNamespace,
		},
	}

	metricsNN := types.NamespacedName{
		Name:      devConfigName + "-node-metrics",
		Namespace: devConfigNamespace,
	}

	nn := types.NamespacedName{
		Name:      devConfigName,
		Namespace: devConfigNamespace,
	}

	DescribeTable("finalizer good flow", func(nodeMetricsDSExists,
		kmmModuleExists bool) {
		expectedDevConfig := devConfig.DeepCopy()
		if nodeMetricsDSExists {
			kubeClient.EXPECT().Get(ctx, metricsNN, gomock.Any()).Return(nil)
			kubeClient.EXPECT().Delete(ctx, gomock.Any()).Return(nil)
			goto executeTestFunction
		}
		kubeClient.EXPECT().Get(ctx, metricsNN, gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "dsName"))
		if kmmModuleExists {
			kubeClient.EXPECT().Get(ctx, nn, gomock.Any()).Return(nil)
			kubeClient.EXPECT().Delete(ctx, gomock.Any()).Return(nil)
			goto executeTestFunction
		}
		kubeClient.EXPECT().Get(ctx, nn, gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "modName"))
		upgradeHelper.EXPECT().RemoveUpgradeLabels(ctx, devConfig).Return(nil)
		expectedDevConfig.SetFinalizers([]string{})
		controllerutil.AddFinalizer(devConfig, deviceConfigFinalizer)
		kubeClient.EXPECT().Patch(ctx, expectedDevConfig, gomock.Any()).Return(nil)

	executeTestFunction:
		err := dcrh.finalizeDeviceConfig(ctx, devConfig)
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("node metrics daemonset exists", true, false),
		Entry("kmm module exists", false, true),
		Entry("node metrics daemonset and kmm module do not exist", false, false),
	)
})

var _ = Describe("handleKMMModule", func() {
	var (
		kubeClient *mock_client.MockClient
		kmmHelper  *kmmmodule.MockKMMModuleAPI
		dcrh       deviceConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		kmmHelper = kmmmodule.NewMockKMMModuleAPI(ctrl)
		dcrh = newDeviceConfigReconcilerHelper(kubeClient, kmmHelper, nil, nil, nil, nil)
	})

	ctx := context.Background()
	devConfig := &awslabsv1alpha1.DeviceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devConfigName,
			Namespace: devConfigNamespace,
		},
	}

	It("KMM Module does not exist", func() {
		newMod := &kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: devConfig.Namespace,
				Name:      devConfig.Name,
			},
		}
		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "whatever")),
			kmmHelper.EXPECT().SetKMMModuleAsDesired(newMod, devConfig).Return(nil),
			kubeClient.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := dcrh.handleKMMModule(ctx, devConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	It("KMM Module exists", func() {
		existingMod := &kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: devConfig.Namespace,
				Name:      devConfig.Name,
			},
		}
		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Do(
				func(_ interface{}, _ interface{}, mod *kmmv1beta1.Module, _ ...client.GetOption) {
					mod.Name = devConfig.Name
					mod.Namespace = devConfig.Namespace
				},
			),
			kmmHelper.EXPECT().SetKMMModuleAsDesired(existingMod, devConfig).Return(nil),
		)

		err := dcrh.handleKMMModule(ctx, devConfig)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("handleModuleVersionUpgrade", func() {
	var (
		kubeClient    *mock_client.MockClient
		upgradeHelper *upgrade.MockUpgradeAPI
		dcrh          deviceConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		upgradeHelper = upgrade.NewMockUpgradeAPI(ctrl)
		dcrh = newDeviceConfigReconcilerHelper(kubeClient, nil, upgradeHelper, nil, nil, nil)
	})

	ctx := context.Background()
	devConfig := &awslabsv1alpha1.DeviceConfig{
		Spec: awslabsv1alpha1.DeviceConfigSpec{
			DriverVersion: "some verison",
		},
	}
	targetedNodes := []corev1.Node{
		{},
		{},
	}
	upgradedNode := &corev1.Node{}
	nodeForUpgrade := &corev1.Node{}

	DescribeTable("upgrade good and error flow", func(getTargetedNodesError,
		uncordonUpgradedNodeError,
		cordonNodeForUpgradeError,
		kickoffUpgradeError bool) {
		if getTargetedNodesError {
			upgradeHelper.EXPECT().GetTargetedNodes(ctx, devConfig).Return(nil, fmt.Errorf("some error"))
			goto executeTestFunction
		}
		upgradeHelper.EXPECT().GetTargetedNodes(ctx, devConfig).Return(targetedNodes, nil)
		upgradeHelper.EXPECT().GetUpgradedNode(ctx, devConfig, targetedNodes).Return(upgradedNode)
		if uncordonUpgradedNodeError {
			upgradeHelper.EXPECT().UncordonUpgradedNode(ctx, upgradedNode).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		upgradeHelper.EXPECT().UncordonUpgradedNode(ctx, upgradedNode).Return(nil)
		upgradeHelper.EXPECT().GetNodeForUpgrade(ctx, devConfig, targetedNodes).Return(nodeForUpgrade)
		if cordonNodeForUpgradeError {
			upgradeHelper.EXPECT().CordonNodeForUpgrade(ctx, devConfig, nodeForUpgrade).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		upgradeHelper.EXPECT().CordonNodeForUpgrade(ctx, devConfig, nodeForUpgrade).Return(nil)
		if kickoffUpgradeError {
			upgradeHelper.EXPECT().KickoffUpgrade(ctx, devConfig, nodeForUpgrade).Return(fmt.Errorf("some error"))
			goto executeTestFunction
		}
		upgradeHelper.EXPECT().KickoffUpgrade(ctx, devConfig, nodeForUpgrade).Return(nil)

	executeTestFunction:

		err := dcrh.handleModuleVersionUpgrade(ctx, devConfig)
		if getTargetedNodesError || uncordonUpgradedNodeError || cordonNodeForUpgradeError || kickoffUpgradeError {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
	},
		Entry("good flow, no errors", false, false, false, false),
		Entry("getTargetedNodes failed", true, false, false, false),
		Entry("uncordonUpgradedNode failed", false, true, false, false),
		Entry("cordonNodeForUpgrade failed", false, false, true, false),
		Entry("kickoffUpgrade failed", false, false, false, true),
	)

	It("driverVersion is not defined in the Spec", func() {
		devConfig := &awslabsv1alpha1.DeviceConfig{}
		err := dcrh.handleModuleVersionUpgrade(ctx, devConfig)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("handleCustomScheduler", func() {
	var (
		kubeClient    *mock_client.MockClient
		dcrh          deviceConfigReconcilerHelperAPI
		mockCDHandler *customscheduler.MockCustomScheduler
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		mockCDHandler = customscheduler.NewMockCustomScheduler(ctrl)
		dcrh = newDeviceConfigReconcilerHelper(kubeClient, nil, nil, mockCDHandler, nil, scheme)
	})

	ctx := context.Background()
	devConfig := &awslabsv1alpha1.DeviceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devConfigName,
			Namespace: devConfigNamespace,
		},
	}

	It("good flow", func() {
		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, dp *appsv1.Deployment, _ ...client.GetOption) error {
					return nil
				},
			),
			mockCDHandler.EXPECT().SetCustomSchedulerAsDesired(gomock.Any(), devConfig),
			kubeClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(nil),
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, dp *appsv1.Deployment, _ ...client.GetOption) error {
					return nil
				},
			),
			mockCDHandler.EXPECT().SetCustomSchedulerExtensionAsDesired(gomock.Any(), devConfig),
			kubeClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(nil),
		)
		err := dcrh.handleCustomScheduler(ctx, devConfig)

		Expect(err).To(BeNil())
	})

	It("custom scheduler deployment failed", func() {
		kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ interface{}, _ interface{}, dp *appsv1.Deployment, _ ...client.GetOption) error {
				return fmt.Errorf("some error")
			},
		)
		err := dcrh.handleCustomScheduler(ctx, devConfig)
		Expect(err).To(HaveOccurred())
	})

	It("custom scheduler extension deployment failed", func() {
		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, dp *appsv1.Deployment, _ ...client.GetOption) error {
					return nil
				},
			),
			mockCDHandler.EXPECT().SetCustomSchedulerAsDesired(gomock.Any(), devConfig),
			kubeClient.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).Return(nil),
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, dp *appsv1.Deployment, _ ...client.GetOption) error {
					return fmt.Errorf("some error")
				},
			),
		)
		err := dcrh.handleCustomScheduler(ctx, devConfig)
		Expect(err).To(HaveOccurred())
	})

})

var _ = Describe("handleNodeMetrics", func() {
	var (
		kubeClient        *mock_client.MockClient
		nodeMetricsHelper *nodemetrics.MockNodeMetrics
		dcrh              deviceConfigReconcilerHelperAPI
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = mock_client.NewMockClient(ctrl)
		nodeMetricsHelper = nodemetrics.NewMockNodeMetrics(ctrl)
		dcrh = newDeviceConfigReconcilerHelper(kubeClient, nil, nil, nil, nodeMetricsHelper, nil)
	})

	ctx := context.Background()
	devConfig := &awslabsv1alpha1.DeviceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devConfigName,
			Namespace: devConfigNamespace,
		},
	}

	It("NodeMetrics DaemonSet does not exist", func() {
		newDS := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: devConfig.Namespace, Name: devConfig.Name + "-node-metrics"},
		}

		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(k8serrors.NewNotFound(schema.GroupResource{}, "whatever")),
			nodeMetricsHelper.EXPECT().SetNodeMetricsAsDesired(newDS, devConfig).Return(nil),
			kubeClient.EXPECT().Create(ctx, gomock.Any()).Return(nil),
		)

		err := dcrh.handleNodeMetrics(ctx, devConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	It("NodeMetrcis DaemonSet exists", func() {
		existingDS := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: devConfig.Namespace, Name: devConfig.Name + "-node-metrics"},
		}

		gomock.InOrder(
			kubeClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Do(
				func(_ interface{}, _ interface{}, ds *appsv1.DaemonSet, _ ...client.GetOption) {
					ds.Name = devConfig.Name + "-node-metrics"
					ds.Namespace = devConfig.Namespace
				},
			),
			nodeMetricsHelper.EXPECT().SetNodeMetricsAsDesired(existingDS, devConfig).Return(nil),
		)

		err := dcrh.handleNodeMetrics(ctx, devConfig)
		Expect(err).ToNot(HaveOccurred())
	})
})
