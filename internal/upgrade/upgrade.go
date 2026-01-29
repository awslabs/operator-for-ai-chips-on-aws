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
	"fmt"

	awslabsv1beta1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1beta1"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/constants"
	kmmlabels "github.com/rh-ecosystem-edge/kernel-module-management/pkg/labels"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type nodeUpgradeState string

const (
	nodeBeforeUpgrade nodeUpgradeState = "nodeBeforeUpgrade"
	nodeInUpgrade     nodeUpgradeState = "nodeInUpgrade"
	nodeUpgraded      nodeUpgradeState = "nodeUpgraded"
)

//go:generate mockgen -source=upgrade.go -package=upgrade -destination=mock_upgrade.go UpgradeAPI
type UpgradeAPI interface {
	GetTargetedNodes(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig) ([]v1.Node, error)
	GetUpgradedNode(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig, nodes []v1.Node) *v1.Node
	UncordonUpgradedNode(ctx context.Context, node *v1.Node) error
	GetNodeForUpgrade(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig, nodes []v1.Node) *v1.Node
	CordonNodeForUpgrade(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig, node *v1.Node) error
	KickoffUpgrade(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig, node *v1.Node) error
	RemoveUpgradeLabels(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig) error
}

type upgradeImpl struct {
	client client.Client
}

func NewUpgradeAPI(client client.Client) UpgradeAPI {
	return &upgradeImpl{
		client: client,
	}
}

func (ui *upgradeImpl) GetTargetedNodes(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig) ([]v1.Node, error) {
	selectedNodes := v1.NodeList{}
	opt := client.MatchingLabels(devConfig.Spec.Selector)
	err := ui.client.List(ctx, &selectedNodes, opt)
	if err != nil {
		return nil, fmt.Errorf("could not list nodes: %v", err)
	}

	return selectedNodes.Items, nil
}

func (ui *upgradeImpl) GetUpgradedNode(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig, nodes []v1.Node) *v1.Node {
	for _, node := range nodes {
		upgradeState := getNodeUpgradeState(node, devConfig)
		if upgradeState == nodeUpgraded && isNodeTainted(node) {
			return &node
		}
	}
	return nil
}

func (ui *upgradeImpl) UncordonUpgradedNode(ctx context.Context, node *v1.Node) error {
	if node == nil {
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info("start uncordon for node", "node name", node.Name)

	upgradeTaint := v1.Taint{
		Key:    constants.UpgradeTaintTolerationKey,
		Value:  "true",
		Effect: v1.TaintEffectNoExecute,
	}

	newTaints := []v1.Taint{}
	// check if node already contains the taint
	for _, taint := range node.Spec.Taints {
		if upgradeTaint.Key == taint.Key && upgradeTaint.Effect == taint.Effect {
			continue
		}
		newTaints = append(newTaints, taint)
	}
	nodeCopy := node.DeepCopy()
	node.Spec.Taints = newTaints
	return ui.client.Patch(ctx, node, client.MergeFrom(nodeCopy))
}

func (ui *upgradeImpl) GetNodeForUpgrade(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig, nodes []v1.Node) *v1.Node {
	var upgradeNode *v1.Node

	for _, node := range nodes {
		upgradeState := getNodeUpgradeState(node, devConfig)
		switch upgradeState {
		case nodeBeforeUpgrade:
			upgradeNode = &node
		case nodeInUpgrade:
			return nil
		case nodeUpgraded:
			continue
		}
	}
	return upgradeNode
}

func (ui *upgradeImpl) CordonNodeForUpgrade(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig, node *v1.Node) error {
	if node == nil {
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info("start cordon for node", "node name", node.Name)

	if isNewNodeForUpgrade(node, devConfig) {
		// no need to cordone if driver was not loaded previously and no workloads are running
		return nil
	}
	upgradeTaint := v1.Taint{
		Key:    constants.UpgradeTaintTolerationKey,
		Value:  "true",
		Effect: v1.TaintEffectNoExecute,
	}

	// check if node already contains the taint
	for _, taint := range node.Spec.Taints {
		if upgradeTaint.Key == taint.Key && upgradeTaint.Effect == taint.Effect {
			return nil
		}
	}
	nodeCopy := node.DeepCopy()
	node.Spec.Taints = append(node.Spec.Taints, upgradeTaint)
	return ui.client.Patch(ctx, node, client.MergeFrom(nodeCopy))
}

func (ui *upgradeImpl) KickoffUpgrade(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig, node *v1.Node) error {
	if node == nil {
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info("start kickoff for node", "node name", node.Name)

	nodeLabels := node.GetLabels()
	if nodeLabels == nil {
		nodeLabels = make(map[string]string)
	}
	nodeCopy := node.DeepCopy()
	nodeLabels[kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)] = devConfig.Spec.DriverVersion
	node.SetLabels(nodeLabels)
	return ui.client.Patch(ctx, node, client.MergeFrom(nodeCopy))
}

func (ui *upgradeImpl) RemoveUpgradeLabels(ctx context.Context, devConfig *awslabsv1beta1.DeviceConfig) error {

	selectedNodes := v1.NodeList{}

	upgradeLabelKey := kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)
	opt := client.MatchingLabels(map[string]string{upgradeLabelKey: devConfig.Spec.DriverVersion})
	err := ui.client.List(ctx, &selectedNodes, opt)
	if err != nil {
		return fmt.Errorf("could not list nodes: %v", err)
	}

	for _, node := range selectedNodes.Items {
		nodeCopy := node.DeepCopy()
		nodeLabels := node.GetLabels()
		delete(nodeLabels, upgradeLabelKey)
		node.SetLabels(nodeLabels)
		err = ui.client.Patch(ctx, &node, client.MergeFrom(nodeCopy))
		if err != nil {
			return fmt.Errorf("failed to remove upgrade label %s from node %s: %v", upgradeLabelKey, node.Name, err)
		}
	}
	return nil
}

func getNodeUpgradeState(node v1.Node, devConfig *awslabsv1beta1.DeviceConfig) nodeUpgradeState {
	nodeLabels := node.GetLabels()
	moduleVersion, ok := nodeLabels[kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)]
	if !ok || moduleVersion != devConfig.Spec.DriverVersion {
		return nodeBeforeUpgrade
	}
	moduleVersionReady, ok := nodeLabels[kmmlabels.GetKernelModuleVersionReadyNodeLabel(devConfig.Namespace, devConfig.Name)]
	if !ok || moduleVersionReady != devConfig.Spec.DriverVersion {
		return nodeInUpgrade
	}
	return nodeUpgraded
}

func isNewNodeForUpgrade(node *v1.Node, devConfig *awslabsv1beta1.DeviceConfig) bool {
	nodeLabels := node.GetLabels()
	_, ok := nodeLabels[kmmlabels.GetModuleVersionLabelName(devConfig.Namespace, devConfig.Name)]
	return !ok
}

func isNodeTainted(node v1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == constants.UpgradeTaintTolerationKey {
			return true
		}
	}
	return false
}
