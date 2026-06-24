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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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

	It("should return error when DaemonSet pointer is nil", func() {
		devConfig := &awslabsv1beta1.DeviceConfig{
			Spec: awslabsv1beta1.DeviceConfigSpec{
				DRADriverImage: "test-dra-driver-image:latest",
			},
		}

		err := dd.SetDRADriverAsDesired(nil, devConfig)
		Expect(err).To(HaveOccurred())
	})
})
