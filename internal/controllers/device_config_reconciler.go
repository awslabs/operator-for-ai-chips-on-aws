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
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/customscheduler"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/filter"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/kmmmodule"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/nodemetrics"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/upgrade"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	DeviceConfigReconcilerName = "DriverAndPluginReconciler"
	deviceConfigFinalizer      = "awslabs.node.kubernetes.io/deviceconfig-finalizer"
)

// ModuleReconciler reconciles a Module object
type DeviceConfigReconciler struct {
	helper deviceConfigReconcilerHelperAPI
	filter *filter.Filter
}

func NewDeviceConfigReconciler(
	client client.Client,
	kmmHandler kmmmodule.KMMModuleAPI,
	upgradeHandler upgrade.UpgradeAPI,
	csHandler customscheduler.CustomScheduler,
	nmHandler nodemetrics.NodeMetrics,
	filter *filter.Filter,
	scheme *runtime.Scheme) *DeviceConfigReconciler {
	helper := newDeviceConfigReconcilerHelper(client, kmmHandler, upgradeHandler, csHandler, nmHandler, scheme)
	return &DeviceConfigReconciler{
		helper: helper,
		filter: filter,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awslabsv1alpha1.DeviceConfig{}).
		Owns(&kmmv1beta1.Module{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.filter.FindDeviceConfigForNodeChange),
			builder.WithPredicates(r.filter.GetNodePredicate()),
		).
		Named(DeviceConfigReconcilerName).
		Complete(
			reconcile.AsReconciler[*awslabsv1alpha1.DeviceConfig](mgr.GetClient(), r),
		)
}

//+kubebuilder:rbac:groups=k8s.aws,resources=deviceconfigs,verbs=get;list;watch;create;patch;update
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules,verbs=get;list;watch;create;patch;update;delete
//+kubebuilder:rbac:groups=k8s.aws,resources=deviceconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=create;delete;get;list;patch;watch;create
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;patch;watch

func (r *DeviceConfigReconciler) Reconcile(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := log.FromContext(ctx).WithValues("namespace", devConfig.Namespace, "name", devConfig.Name)
	if devConfig.GetDeletionTimestamp() != nil {
		// DeviceConfig is being deleted
		err := r.helper.finalizeDeviceConfig(ctx, devConfig)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to finalize DeviceConfig: %v", err)
		}
		return ctrl.Result{}, nil
	}

	err := r.helper.setFinalizer(ctx, devConfig)
	if err != nil {
		return res, fmt.Errorf("failed to set finalizer for DeviceConfig: %v", err)
	}

	logger.Info("start KMM reconciliation")
	err = r.helper.handleKMMModule(ctx, devConfig)
	if err != nil {
		return res, fmt.Errorf("failed to handle KMM module for DeviceConfig: %v", err)
	}

	logger.Info("start rolling upgrade reconciliation")
	err = r.helper.handleModuleVersionUpgrade(ctx, devConfig)
	if err != nil {
		return res, fmt.Errorf("failed to handle KMM module version upgrade for DeviceConfig: %v", err)
	}

	logger.Info("start custom scheduler reconciliation")
	err = r.helper.handleCustomScheduler(ctx, devConfig)
	if err != nil {
		return res, fmt.Errorf("failed to handle customScheduler for DeviceConfig: %v", err)
	}

	logger.Info("start metrics reconciliation")
	err = r.helper.handleNodeMetrics(ctx, devConfig)
	if err != nil {
		return res, fmt.Errorf("failed to handle node metrics for DeviceConfig: %v", err)
	}
	// [TODO] add status handling for DeviceConfig
	return res, nil
}

//go:generate mockgen -source=device_config_reconciler.go -package=controllers -destination=mock_device_config_reconciler.go deviceConfigReconcilerHelperAPI
type deviceConfigReconcilerHelperAPI interface {
	finalizeDeviceConfig(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error
	setFinalizer(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error
	handleKMMModule(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error
	handleModuleVersionUpgrade(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error
	handleCustomScheduler(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error
	handleNodeMetrics(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error
}

type deviceConfigReconcilerHelper struct {
	client         client.Client
	kmmHandler     kmmmodule.KMMModuleAPI
	upgradeHandler upgrade.UpgradeAPI
	csHandler      customscheduler.CustomScheduler
	nmHandler      nodemetrics.NodeMetrics
	scheme         *runtime.Scheme
}

func newDeviceConfigReconcilerHelper(client client.Client,
	kmmHandler kmmmodule.KMMModuleAPI,
	upgradeHandler upgrade.UpgradeAPI,
	csHandler customscheduler.CustomScheduler,
	nmHandler nodemetrics.NodeMetrics,
	scheme *runtime.Scheme) deviceConfigReconcilerHelperAPI {
	return &deviceConfigReconcilerHelper{
		client:         client,
		kmmHandler:     kmmHandler,
		upgradeHandler: upgradeHandler,
		csHandler:      csHandler,
		nmHandler:      nmHandler,
		scheme:         scheme,
	}
}

func (dcrh *deviceConfigReconcilerHelper) setFinalizer(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error {
	if controllerutil.ContainsFinalizer(devConfig, deviceConfigFinalizer) {
		return nil
	}

	devConfigCopy := devConfig.DeepCopy()
	controllerutil.AddFinalizer(devConfig, deviceConfigFinalizer)
	return dcrh.client.Patch(ctx, devConfig, client.MergeFrom(devConfigCopy))
}

func (dcrh *deviceConfigReconcilerHelper) finalizeDeviceConfig(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error {
	logger := log.FromContext(ctx)

	nmDS := appsv1.DaemonSet{}
	namespacedName := types.NamespacedName{
		Namespace: devConfig.Namespace,
		Name:      devConfig.Name + "-node-metrics",
	}

	err := dcrh.client.Get(ctx, namespacedName, &nmDS)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get nodemetrics daemonset %s: %v", namespacedName, err)
		}
	} else {
		logger.Info("deleting nodemetrics daemonset", "daemonset", namespacedName)
		return dcrh.client.Delete(ctx, &nmDS)
	}

	mod := kmmv1beta1.Module{}
	namespacedName = types.NamespacedName{
		Namespace: devConfig.Namespace,
		Name:      devConfig.Name,
	}
	err = dcrh.client.Get(ctx, namespacedName, &mod)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the requested Module %s: %v", namespacedName, err)
		}
	} else {
		logger.Info("deleting KMM Module", "module", namespacedName)
		return dcrh.client.Delete(ctx, &mod)
	}

	err = dcrh.upgradeHandler.RemoveUpgradeLabels(ctx, devConfig)
	if err != nil {
		return fmt.Errorf("failed to remove upgrade labels from nodes %s: %v", namespacedName, err)
	}

	logger.Info("module and upgrade labels already deleted, removing finalizer", "module", namespacedName)
	devConfigCopy := devConfig.DeepCopy()
	controllerutil.RemoveFinalizer(devConfig, deviceConfigFinalizer)
	return dcrh.client.Patch(ctx, devConfig, client.MergeFrom(devConfigCopy))
}

func (dcrh *deviceConfigReconcilerHelper) handleKMMModule(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error {
	kmmMod := &kmmv1beta1.Module{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: devConfig.Namespace,
			Name:      devConfig.Name,
		},
	}
	logger := log.FromContext(ctx)
	opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, kmmMod, func() error {
		return dcrh.kmmHandler.SetKMMModuleAsDesired(kmmMod, devConfig)
	})

	if err == nil {
		logger.Info("Reconciled KMM Module", "name", kmmMod.Name, "result", opRes)
	}

	return err

}

func (dcrh *deviceConfigReconcilerHelper) handleModuleVersionUpgrade(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error {
	// check if rolling upgrade should be supported
	if devConfig.Spec.DriverVersion == "" {
		return nil
	}
	logger := log.FromContext(ctx)
	targetedNodes, err := dcrh.upgradeHandler.GetTargetedNodes(ctx, devConfig)
	if err != nil {
		return fmt.Errorf("failed to get nodes targeted by the DeviceConfig %s/%s: %v", devConfig.Namespace, devConfig.Name, err)
	}

	logger.Info("targeted nodes for rolling upgrade", "num nodes", len(targetedNodes))

	node := dcrh.upgradeHandler.GetUpgradedNode(ctx, devConfig, targetedNodes)

	err = dcrh.upgradeHandler.UncordonUpgradedNode(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to finzalize upgraded nodes for DeviceConfig %s/%s: %v", devConfig.Namespace, devConfig.Name, err)
	}

	node = dcrh.upgradeHandler.GetNodeForUpgrade(ctx, devConfig, targetedNodes)

	err = dcrh.upgradeHandler.CordonNodeForUpgrade(ctx, devConfig, node)
	if err != nil {
		return fmt.Errorf("failed to cordon node %s for DeviceConfig %s/%s: %v", node.Name, devConfig.Namespace, devConfig.Name, err)
	}

	return dcrh.upgradeHandler.KickoffUpgrade(ctx, devConfig, node)
}

func (dcrh *deviceConfigReconcilerHelper) handleCustomScheduler(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error {
	csDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: devConfig.Namespace, Name: devConfig.Name + "-custom-scheduler"},
	}
	logger := log.FromContext(ctx)
	opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, csDep, func() error {
		dcrh.csHandler.SetCustomSchedulerAsDesired(csDep, devConfig)
		return controllerutil.SetControllerReference(devConfig, csDep, dcrh.scheme)
	})
	if err != nil {
		return fmt.Errorf("failed to create/patch custom scheduler deployment: %v", err)
	}

	logger.Info("Reconciled custom scheduler", "namespace", csDep.Namespace, "name", csDep.Name, "result", opRes)

	cseDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: devConfig.Namespace, Name: devConfig.Name + "-custom-scheduler-extension"},
	}
	opRes, err = controllerutil.CreateOrPatch(ctx, dcrh.client, cseDep, func() error {
		dcrh.csHandler.SetCustomSchedulerExtensionAsDesired(cseDep, devConfig)
		return controllerutil.SetControllerReference(devConfig, cseDep, dcrh.scheme)
	})

	if err == nil {
		logger.Info("Reconciled custom scheduler extension", "namespace", cseDep.Namespace, "name", cseDep.Name, "result", opRes)
	}

	return err
}

func (dcrh *deviceConfigReconcilerHelper) handleNodeMetrics(ctx context.Context, devConfig *awslabsv1alpha1.DeviceConfig) error {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: devConfig.Namespace, Name: devConfig.Name + "-node-metrics"},
	}
	logger := log.FromContext(ctx)
	opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, ds, func() error {
		return dcrh.nmHandler.SetNodeMetricsAsDesired(ds, devConfig)
	})

	if err == nil {
		logger.Info("Reconciled node metrics", "namespace", ds.Namespace, "name", ds.Name, "result", opRes)
	}

	return err
}
