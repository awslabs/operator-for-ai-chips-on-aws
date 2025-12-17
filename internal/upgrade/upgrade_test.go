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

package upgrade

import (
	"context"
	"errors"

	awslabsv1alpha1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1alpha1"
	mock_client "github.com/awslabs/operator-for-ai-chips-on-aws/internal/client"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kmmlabels "github.com/rh-ecosystem-edge/kernel-module-management/pkg/labels"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("NewUpgradeAPI", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *mock_client.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mock_client.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should create a new UpgradeAPI instance", func() {
		api := NewUpgradeAPI(mockClient)
		Expect(api).NotTo(BeNil())
		impl, ok := api.(*upgradeImpl)
		Expect(ok).To(BeTrue())
		Expect(impl.client).To(Equal(mockClient))
	})
})

var _ = Describe("GetTargetedNodes", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *mock_client.MockClient
		upgradeAPI UpgradeAPI
		ctx        context.Context
		devConfig  *awslabsv1alpha1.DeviceConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mock_client.NewMockClient(ctrl)
		upgradeAPI = NewUpgradeAPI(mockClient)
		ctx = context.Background()
		devConfig = &awslabsv1alpha1.DeviceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-device-config",
				Namespace: "test-namespace",
			},
			Spec: awslabsv1alpha1.DeviceConfigSpec{
				Selector: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should successfully list nodes with matching labels", func() {
		expectedNodes := []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
					Labels: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
		}

		mockClient.EXPECT().
			List(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
				nodeList := list.(*v1.NodeList)
				nodeList.Items = expectedNodes
				return nil
			})

		nodes, err := upgradeAPI.GetTargetedNodes(ctx, devConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodes).To(HaveLen(2))
		Expect(nodes[0].Name).To(Equal("node1"))
		Expect(nodes[1].Name).To(Equal("node2"))
	})

	It("should return empty list when no nodes match", func() {
		mockClient.EXPECT().
			List(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
				nodeList := list.(*v1.NodeList)
				nodeList.Items = []v1.Node{}
				return nil
			})

		nodes, err := upgradeAPI.GetTargetedNodes(ctx, devConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodes).To(BeEmpty())
	})

	It("should return error when client.List fails", func() {
		expectedError := errors.New("list error")
		mockClient.EXPECT().
			List(ctx, gomock.Any(), gomock.Any()).
			Return(expectedError)

		nodes, err := upgradeAPI.GetTargetedNodes(ctx, devConfig)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not list nodes"))
		Expect(nodes).To(BeNil())
	})
})

var _ = Describe("GetUpgradedNode", func() {
	var (
		upgradeAPI UpgradeAPI
		ctx        context.Context
		devConfig  *awslabsv1alpha1.DeviceConfig
	)

	BeforeEach(func() {
		upgradeAPI = NewUpgradeAPI(nil)
		ctx = context.Background()
		devConfig = &awslabsv1alpha1.DeviceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-device-config",
				Namespace: "test-namespace",
			},
			Spec: awslabsv1alpha1.DeviceConfigSpec{
				DriverVersion: "v1.0.0",
			},
		}
	})

	It("should return node that is fully upgraded", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)
		moduleVersionReadyLabel := kmmlabels.GetKernelModuleVersionReadyNodeLabel(devConfig.Namespace, devConfig.Name)

		nodes := []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						moduleVersionLabel:      "v0.9.0",
						moduleVersionReadyLabel: "v0.9.0",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
					Labels: map[string]string{
						moduleVersionLabel:      devConfig.Spec.DriverVersion,
						moduleVersionReadyLabel: devConfig.Spec.DriverVersion,
					},
				},
			},
		}

		upgradedNode := upgradeAPI.GetUpgradedNode(ctx, devConfig, nodes)
		Expect(upgradedNode).NotTo(BeNil())
		Expect(upgradedNode.Name).To(Equal("node2"))
	})

	It("should return nil when no node is upgraded", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)

		nodes := []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						moduleVersionLabel: "v0.9.0",
					},
				},
			},
		}

		upgradedNode := upgradeAPI.GetUpgradedNode(ctx, devConfig, nodes)
		Expect(upgradedNode).To(BeNil())
	})

	It("should return nil when nodes list is empty", func() {
		nodes := []v1.Node{}
		upgradedNode := upgradeAPI.GetUpgradedNode(ctx, devConfig, nodes)
		Expect(upgradedNode).To(BeNil())
	})
})

var _ = Describe("UncordonUpgradedNode", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *mock_client.MockClient
		upgradeAPI UpgradeAPI
		ctx        context.Context
		node       *v1.Node
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mock_client.NewMockClient(ctrl)
		upgradeAPI = NewUpgradeAPI(mockClient)
		ctx = context.Background()

		node = &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
			Spec: v1.NodeSpec{
				Taints: []v1.Taint{
					{
						Key:    constants.UpgradeTaintTolerationKey,
						Value:  "true",
						Effect: v1.TaintEffectNoExecute,
					},
					{
						Key:    "other-taint",
						Value:  "other-value",
						Effect: v1.TaintEffectNoSchedule,
					},
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should successfully remove upgrade taint from node", func() {
		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchedNode := obj.(*v1.Node)
				Expect(patchedNode.Spec.Taints).To(HaveLen(1))
				Expect(patchedNode.Spec.Taints[0].Key).To(Equal("other-taint"))
				return nil
			})

		err := upgradeAPI.UncordonUpgradedNode(ctx, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not fail when upgrade taint is not present", func() {
		node.Spec.Taints = []v1.Taint{
			{
				Key:    "other-taint",
				Value:  "other-value",
				Effect: v1.TaintEffectNoSchedule,
			},
		}

		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchedNode := obj.(*v1.Node)
				Expect(patchedNode.Spec.Taints).To(HaveLen(1))
				return nil
			})

		err := upgradeAPI.UncordonUpgradedNode(ctx, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should handle node with no taints", func() {
		node.Spec.Taints = []v1.Taint{}

		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchedNode := obj.(*v1.Node)
				Expect(patchedNode.Spec.Taints).To(BeEmpty())
				return nil
			})

		err := upgradeAPI.UncordonUpgradedNode(ctx, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return error when Patch fails", func() {
		expectedError := errors.New("patch error")
		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			Return(expectedError)

		err := upgradeAPI.UncordonUpgradedNode(ctx, node)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(expectedError))
	})

	It("should return nil when node is nil", func() {
		err := upgradeAPI.UncordonUpgradedNode(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("GetNodeForUpgrade", func() {
	var (
		upgradeAPI UpgradeAPI
		ctx        context.Context
		devConfig  *awslabsv1alpha1.DeviceConfig
	)

	BeforeEach(func() {
		upgradeAPI = NewUpgradeAPI(nil)
		ctx = context.Background()
		devConfig = &awslabsv1alpha1.DeviceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-device-config",
				Namespace: "test-namespace",
			},
			Spec: awslabsv1alpha1.DeviceConfigSpec{
				DriverVersion: "v1.0.0",
			},
		}
	})

	It("should return node that needs upgrade", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)

		nodes := []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						moduleVersionLabel: "v0.9.0",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
					Labels: map[string]string{
						moduleVersionLabel: "v0.8.0",
					},
				},
			},
		}

		upgradeNode := upgradeAPI.GetNodeForUpgrade(ctx, devConfig, nodes)
		Expect(upgradeNode).NotTo(BeNil())
		Expect(upgradeNode.Name).To(Equal("node2"))
	})

	It("should return nil when node is in upgrade", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)
		moduleVersionReadyLabel := kmmlabels.GetKernelModuleVersionReadyNodeLabel(devConfig.Namespace, devConfig.Name)

		nodes := []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						moduleVersionLabel:      devConfig.Spec.DriverVersion,
						moduleVersionReadyLabel: "v0.9.0",
					},
				},
			},
		}

		upgradeNode := upgradeAPI.GetNodeForUpgrade(ctx, devConfig, nodes)
		Expect(upgradeNode).To(BeNil())
	})

	It("should skip upgraded nodes", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)
		moduleVersionReadyLabel := kmmlabels.GetKernelModuleVersionReadyNodeLabel(devConfig.Namespace, devConfig.Name)

		nodes := []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
					Labels: map[string]string{
						moduleVersionLabel:      devConfig.Spec.DriverVersion,
						moduleVersionReadyLabel: devConfig.Spec.DriverVersion,
					},
				},
			},
		}

		upgradeNode := upgradeAPI.GetNodeForUpgrade(ctx, devConfig, nodes)
		Expect(upgradeNode).To(BeNil())
	})

	It("should return nil when nodes list is empty", func() {
		nodes := []v1.Node{}
		upgradeNode := upgradeAPI.GetNodeForUpgrade(ctx, devConfig, nodes)
		Expect(upgradeNode).To(BeNil())
	})

	It("should return node without module version label", func() {
		nodes := []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node1",
					Labels: map[string]string{},
				},
			},
		}

		upgradeNode := upgradeAPI.GetNodeForUpgrade(ctx, devConfig, nodes)
		Expect(upgradeNode).NotTo(BeNil())
		Expect(upgradeNode.Name).To(Equal("node1"))
	})
})

var _ = Describe("CordonNodeForUpgrade", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *mock_client.MockClient
		upgradeAPI UpgradeAPI
		ctx        context.Context
		node       *v1.Node
		devConfig  *awslabsv1alpha1.DeviceConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mock_client.NewMockClient(ctrl)
		upgradeAPI = NewUpgradeAPI(mockClient)
		ctx = context.Background()
		node = &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-node",
				Labels: map[string]string{kmmlabels.GetModuleVersionLabelName("test-namespace", "test-device-config"): "v1.0.0"},
			},
			Spec: v1.NodeSpec{
				Taints: []v1.Taint{},
			},
		}
		devConfig = &awslabsv1alpha1.DeviceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-device-config",
				Namespace: "test-namespace",
			},
			Spec: awslabsv1alpha1.DeviceConfigSpec{
				DriverVersion: "v1.0.0",
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("node should not be tainted if this is the first time driver is deployed", func() {
		node.Labels = map[string]string{}
		err := upgradeAPI.CordonNodeForUpgrade(ctx, devConfig, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should successfully add upgrade taint to node", func() {
		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchedNode := obj.(*v1.Node)
				Expect(patchedNode.Spec.Taints).To(HaveLen(1))
				Expect(patchedNode.Spec.Taints[0].Key).To(Equal(constants.UpgradeTaintTolerationKey))
				Expect(patchedNode.Spec.Taints[0].Value).To(Equal("true"))
				Expect(patchedNode.Spec.Taints[0].Effect).To(Equal(v1.TaintEffectNoExecute))
				return nil
			})

		err := upgradeAPI.CordonNodeForUpgrade(ctx, devConfig, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not add taint if it already exists", func() {
		node.Spec.Taints = []v1.Taint{
			{
				Key:    constants.UpgradeTaintTolerationKey,
				Value:  "true",
				Effect: v1.TaintEffectNoExecute,
			},
		}

		err := upgradeAPI.CordonNodeForUpgrade(ctx, devConfig, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should preserve existing taints when adding upgrade taint", func() {
		node.Spec.Taints = []v1.Taint{
			{
				Key:    "other-taint",
				Value:  "other-value",
				Effect: v1.TaintEffectNoSchedule,
			},
		}

		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchedNode := obj.(*v1.Node)
				Expect(patchedNode.Spec.Taints).To(HaveLen(2))
				hasUpgradeTaint := false
				hasOtherTaint := false
				for _, taint := range patchedNode.Spec.Taints {
					if taint.Key == constants.UpgradeTaintTolerationKey {
						hasUpgradeTaint = true
					}
					if taint.Key == "other-taint" {
						hasOtherTaint = true
					}
				}
				Expect(hasUpgradeTaint).To(BeTrue())
				Expect(hasOtherTaint).To(BeTrue())
				return nil
			})

		err := upgradeAPI.CordonNodeForUpgrade(ctx, devConfig, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return error when Patch fails", func() {
		expectedError := errors.New("patch error")
		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			Return(expectedError)

		err := upgradeAPI.CordonNodeForUpgrade(ctx, devConfig, node)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(expectedError))
	})

	It("should return nil when node is nil", func() {
		err := upgradeAPI.CordonNodeForUpgrade(ctx, devConfig, nil)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("KickoffUpgrade", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *mock_client.MockClient
		upgradeAPI UpgradeAPI
		ctx        context.Context
		devConfig  *awslabsv1alpha1.DeviceConfig
		node       *v1.Node
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mock_client.NewMockClient(ctrl)
		upgradeAPI = NewUpgradeAPI(mockClient)
		ctx = context.Background()
		devConfig = &awslabsv1alpha1.DeviceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-device-config",
				Namespace: "test-namespace",
			},
			Spec: awslabsv1alpha1.DeviceConfigSpec{
				DriverVersion: "v1.0.0",
			},
		}

		node = &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-node",
				Labels: map[string]string{},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should successfully set module version label on node", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)

		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchedNode := obj.(*v1.Node)
				Expect(patchedNode.Labels).To(HaveKey(moduleVersionLabel))
				Expect(patchedNode.Labels[moduleVersionLabel]).To(Equal(devConfig.Spec.DriverVersion))
				return nil
			})

		err := upgradeAPI.KickoffUpgrade(ctx, devConfig, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should preserve existing labels when setting module version label", func() {
		node.Labels = map[string]string{
			"existing-label": "existing-value",
		}
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)

		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchedNode := obj.(*v1.Node)
				Expect(patchedNode.Labels).To(HaveKey("existing-label"))
				Expect(patchedNode.Labels["existing-label"]).To(Equal("existing-value"))
				Expect(patchedNode.Labels).To(HaveKey(moduleVersionLabel))
				Expect(patchedNode.Labels[moduleVersionLabel]).To(Equal(devConfig.Spec.DriverVersion))
				return nil
			})

		err := upgradeAPI.KickoffUpgrade(ctx, devConfig, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should handle node with nil labels", func() {
		node.Labels = nil
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)

		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchedNode := obj.(*v1.Node)
				Expect(patchedNode.Labels).To(HaveKey(moduleVersionLabel))
				Expect(patchedNode.Labels[moduleVersionLabel]).To(Equal(devConfig.Spec.DriverVersion))
				return nil
			})

		err := upgradeAPI.KickoffUpgrade(ctx, devConfig, node)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return error when Patch fails", func() {
		expectedError := errors.New("patch error")
		mockClient.EXPECT().
			Patch(ctx, gomock.Any(), gomock.Any()).
			Return(expectedError)

		err := upgradeAPI.KickoffUpgrade(ctx, devConfig, node)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(expectedError))
	})
})

var _ = Describe("getNodeUpgradeState", func() {
	var (
		devConfig *awslabsv1alpha1.DeviceConfig
	)

	BeforeEach(func() {
		devConfig = &awslabsv1alpha1.DeviceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-device-config",
				Namespace: "test-namespace",
			},
			Spec: awslabsv1alpha1.DeviceConfigSpec{
				DriverVersion: "v1.0.0",
			},
		}
	})

	It("should return nodeBeforeUpgrade when module version label is missing", func() {
		node := v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-node",
				Labels: map[string]string{},
			},
		}

		state := getNodeUpgradeState(node, devConfig)
		Expect(state).To(Equal(nodeBeforeUpgrade))
	})

	It("should return nodeBeforeUpgrade when module version label doesn't match", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)
		node := v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
				Labels: map[string]string{
					moduleVersionLabel: "v0.9.0",
				},
			},
		}

		state := getNodeUpgradeState(node, devConfig)
		Expect(state).To(Equal(nodeBeforeUpgrade))
	})

	It("should return nodeInUpgrade when module version matches but ready label doesn't", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)
		moduleVersionReadyLabel := kmmlabels.GetKernelModuleVersionReadyNodeLabel(devConfig.Namespace, devConfig.Name)
		node := v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
				Labels: map[string]string{
					moduleVersionLabel:      devConfig.Spec.DriverVersion,
					moduleVersionReadyLabel: "v0.9.0",
				},
			},
		}

		state := getNodeUpgradeState(node, devConfig)
		Expect(state).To(Equal(nodeInUpgrade))
	})

	It("should return nodeInUpgrade when module version matches but ready label is missing", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)
		node := v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
				Labels: map[string]string{
					moduleVersionLabel: devConfig.Spec.DriverVersion,
				},
			},
		}

		state := getNodeUpgradeState(node, devConfig)
		Expect(state).To(Equal(nodeInUpgrade))
	})

	It("should return nodeUpgraded when both labels match", func() {
		moduleVersionLabel := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)
		moduleVersionReadyLabel := kmmlabels.GetKernelModuleVersionReadyNodeLabel(devConfig.Namespace, devConfig.Name)
		node := v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
				Labels: map[string]string{
					moduleVersionLabel:      devConfig.Spec.DriverVersion,
					moduleVersionReadyLabel: devConfig.Spec.DriverVersion,
				},
			},
		}

		state := getNodeUpgradeState(node, devConfig)
		Expect(state).To(Equal(nodeUpgraded))
	})
})
