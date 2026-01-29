package filter

import (
	"context"
	"fmt"

	//"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	awslabsv1beta1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1beta1"
)

type Filter struct {
	client client.Client
}

func New(client client.Client) *Filter {
	return &Filter{
		client: client,
	}
}

func (f *Filter) GetNodePredicate() predicate.Predicate {
	return predicate.Or(
		predicate.LabelChangedPredicate{},
		predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return true },
			UpdateFunc:  func(e event.UpdateEvent) bool { return false },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		},
	)
}

// FindDeviceConfigForNodeChange finds the modules that are affected by node changes that result
// in ModuleReconcilerNodePredicate predicate. First it find all the Module that can run on the node, based
// on the Modules' Selector field and on node's labels. Then, in case NMC for the node exists, it adds all the
// Modules already set in NMC ( in case they were not added in a previous step).
func (f *Filter) FindDeviceConfigForNodeChange(ctx context.Context, node client.Object) []reconcile.Request {
	logger := ctrl.LoggerFrom(ctx).WithValues("node", node.GetName())

	logger.Info("Listing all device configs")

	deviceConfigs := awslabsv1beta1.DeviceConfigList{}

	err := f.client.List(ctx, &deviceConfigs)
	if err != nil {
		logger.Error(err, "could not list device configs")
		return nil
	}

	logger.Info("Listed device configs", "count", len(deviceConfigs.Items))
	nodeLabels := node.GetLabels()
	reqs := make([]reconcile.Request, 0)

	for _, deviceConfig := range deviceConfigs.Items {
		logger := logger.WithValues("device config name", deviceConfig.Name)

		logger.V(1).Info("Processing device config")
		match, err := isObjectSelectedByLabels(nodeLabels, deviceConfig.Spec.Selector)
		if err != nil {
			logger.Error(err, "could not determine if node is selected by device config", "node", node.GetName(), "device config", deviceConfig.Name)
			continue
		}
		if !match {
			logger.V(1).Info("Node labels do not match the device config's selector; skipping")
			continue
		}

		nsn := types.NamespacedName{Name: deviceConfig.Name, Namespace: deviceConfig.Namespace}
		reqs = append(reqs, reconcile.Request{NamespacedName: nsn})
	}

	logger.Info("Adding reconciliation requests", "count", len(reqs))
	logger.V(1).Info("New requests", "requests", reqs)

	return reqs
}

func isObjectSelectedByLabels(objectLabels map[string]string, selectorLabels map[string]string) (bool, error) {
	objectLabelsSet := labels.Set(objectLabels)
	sel := labels.NewSelector()

	for k, v := range selectorLabels {
		requirement, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return false, fmt.Errorf("failed to create new label requirements: %v", err)
		}
		sel = sel.Add(*requirement)
	}

	return sel.Matches(objectLabelsSet), nil
}
