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

package filter

import (
	"context"

	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("DeviceClassToModuleReconcileRequest", func() {
	var f *Filter

	BeforeEach(func() {
		f = New(nil)
	})

	It("should return reconcile request when both labels are present", func() {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-device-class",
				Labels: map[string]string{
					constants.DeviceConfigNameLabel:      "my-config",
					constants.DeviceConfigNamespaceLabel: "my-namespace",
				},
			},
		}

		reqs := f.DeviceClassToModuleReconcileRequest(context.Background(), obj)
		Expect(reqs).To(Equal([]reconcile.Request{
			{NamespacedName: types.NamespacedName{Name: "my-config", Namespace: "my-namespace"}},
		}))
	})

	It("should return nil when name label is missing", func() {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-device-class",
				Labels: map[string]string{
					constants.DeviceConfigNamespaceLabel: "my-namespace",
				},
			},
		}

		reqs := f.DeviceClassToModuleReconcileRequest(context.Background(), obj)
		Expect(reqs).To(BeNil())
	})

	It("should return nil when namespace label is missing", func() {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-device-class",
				Labels: map[string]string{
					constants.DeviceConfigNameLabel: "my-config",
				},
			},
		}

		reqs := f.DeviceClassToModuleReconcileRequest(context.Background(), obj)
		Expect(reqs).To(BeNil())
	})

	It("should return nil when both labels are missing", func() {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-device-class",
				Labels: map[string]string{},
			},
		}

		reqs := f.DeviceClassToModuleReconcileRequest(context.Background(), obj)
		Expect(reqs).To(BeNil())
	})

	It("should return nil when labels are empty strings", func() {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-device-class",
				Labels: map[string]string{
					constants.DeviceConfigNameLabel:      "",
					constants.DeviceConfigNamespaceLabel: "",
				},
			},
		}

		reqs := f.DeviceClassToModuleReconcileRequest(context.Background(), obj)
		Expect(reqs).To(BeNil())
	})
})

var _ = Describe("HasLabel", func() {
	var f *Filter

	BeforeEach(func() {
		f = New(nil)
	})

	It("should return true when object has the label with non-empty value", func() {
		pred := f.HasLabel(constants.DeviceConfigNameLabel)
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constants.DeviceConfigNameLabel: "some-value",
				},
			},
		}

		result := pred.Create(event.CreateEvent{Object: obj})
		Expect(result).To(BeTrue())
	})

	It("should return false when object has the label with empty value", func() {
		pred := f.HasLabel(constants.DeviceConfigNameLabel)
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constants.DeviceConfigNameLabel: "",
				},
			},
		}

		result := pred.Create(event.CreateEvent{Object: obj})
		Expect(result).To(BeFalse())
	})

	It("should return false when object does not have the label", func() {
		pred := f.HasLabel(constants.DeviceConfigNameLabel)
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{},
			},
		}

		result := pred.Create(event.CreateEvent{Object: obj})
		Expect(result).To(BeFalse())
	})

	It("should return false when object has no labels at all", func() {
		pred := f.HasLabel(constants.DeviceConfigNameLabel)
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		result := pred.Create(event.CreateEvent{Object: obj})
		Expect(result).To(BeFalse())
	})
})

// ensure the interface is satisfied (compile-time check)
var _ client.Object = &corev1.ConfigMap{}
