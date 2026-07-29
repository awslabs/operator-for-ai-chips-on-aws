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

package dradriver

import (
	"os"

	awslabsv1beta1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1beta1"
	"github.com/awslabs/operator-for-ai-chips-on-aws/internal/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	resourcev1 "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("SetDRADriverAsDesired", func() {
	dd := NewDRADriver(scheme)

	It("should configure DaemonSet with DRA driver settings", func() {
		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config-dra-driver",
				Namespace: "test-namespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "DaemonSet",
				APIVersion: "apps/v1",
			},
		}
		devConfig := &awslabsv1beta1.DeviceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config",
				Namespace: "test-namespace",
			},
			Spec: awslabsv1beta1.DeviceConfigSpec{
				DRADriverImage: "test-dra-driver-image:latest",
			},
		}

		expectedYAMLFile, err := os.ReadFile("testdata/dra_driver_daemonset.yaml")
		Expect(err).To(BeNil())
		expectedDS := appsv1.DaemonSet{}
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		err = yaml.Unmarshal(expectedJSON, &expectedDS)
		Expect(err).To(BeNil())

		expectedDS.Name = "test-config-dra-driver"
		expectedDS.Namespace = "test-namespace"
		expectedDS.Spec.Template.Spec.NodeSelector = map[string]string{
			"kmm.node.kubernetes.io/test-namespace.test-config.ready": "",
		}

		err = dd.SetDRADriverAsDesired(ds, devConfig)
		Expect(err).To(BeNil())
		Expect(ds.Spec).To(Equal(expectedDS.Spec))
	})

})

var _ = Describe("SetDeviceClassAsDesired", func() {
	dd := NewDRADriver(scheme)

	devConfig := &awslabsv1beta1.DeviceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "test-namespace",
		},
		Spec: awslabsv1beta1.DeviceConfigSpec{
			DRADriverImage: "test-dra-driver-image:latest",
		},
	}

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":             "neuron-dra-driver",
		"app.kubernetes.io/component":        "aws-neuron",
		"app.kubernetes.io/part-of":          "aws-neuron",
		constants.DeviceConfigNameLabel:      "test-config",
		constants.DeviceConfigNamespaceLabel: "test-namespace",
	}

	It("should set default DeviceClass when spec is nil", func() {
		dc := &resourcev1.DeviceClass{
			ObjectMeta: metav1.ObjectMeta{Name: "neuron.aws.com"},
		}

		err := dd.SetDeviceClassAsDesired(dc, devConfig, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(dc.Labels).To(Equal(expectedLabels))
		Expect(dc.Spec.Selectors).To(HaveLen(1))
		Expect(dc.Spec.Selectors[0].CEL).ToNot(BeNil())
		Expect(dc.Spec.Selectors[0].CEL.Expression).To(Equal("device.driver == \"neuron.aws.com\""))
	})

	It("should set DeviceClass from custom spec", func() {
		customSelectors := []resourcev1.DeviceSelector{
			{
				CEL: &resourcev1.CELDeviceSelector{
					Expression: "device.driver == \"custom.driver\"",
				},
			},
		}
		customSpec := &awslabsv1beta1.DeviceClassSpec{
			Name:      "custom-class",
			Selectors: customSelectors,
		}

		dc := &resourcev1.DeviceClass{
			ObjectMeta: metav1.ObjectMeta{Name: "custom-class"},
		}

		err := dd.SetDeviceClassAsDesired(dc, devConfig, customSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(dc.Labels).To(Equal(expectedLabels))
		Expect(dc.Spec.Selectors).To(Equal(customSelectors))
		Expect(dc.Spec.Config).To(BeNil())
	})
})
