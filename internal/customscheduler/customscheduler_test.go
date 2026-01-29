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

package customscheduler

import (
	"os"

	awslabsv1beta1 "github.com/awslabs/operator-for-ai-chips-on-aws/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("SetCustomSchedulerAsDesired", func() {
	cs := NewCustomScheduler(scheme)

	It("should configure deployment with neuron scheduler settings", func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "",
				Namespace: "",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
		}
		devConfig := &awslabsv1beta1.DeviceConfig{
			Spec: awslabsv1beta1.DeviceConfigSpec{
				CustomSchedulerImage: "test-scheduler-image:latest",
			},
		}

		expectedYAMLFile, err := os.ReadFile("testdata/custom_scheduler_deployment.yaml")
		Expect(err).To(BeNil())
		expectedDeployment := appsv1.Deployment{}
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		err = yaml.Unmarshal(expectedJSON, &expectedDeployment)
		Expect(err).To(BeNil())

		cs.SetCustomSchedulerAsDesired(deployment, devConfig)
		Expect(*deployment).To(Equal(expectedDeployment))
	})
})

var _ = Describe("SetCustomSchedulerExtensionAsDesired", func() {
	cs := NewCustomScheduler(scheme)

	It("should configure deployment with neuron scheduler extension settings", func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "",
				Namespace: "",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
		}
		devConfig := &awslabsv1beta1.DeviceConfig{
			Spec: awslabsv1beta1.DeviceConfigSpec{
				SchedulerExtensionImage: "test-extension-image:latest",
			},
		}

		expectedYAMLFile, err := os.ReadFile("testdata/custom_scheduler_extension_deployment.yaml")
		Expect(err).To(BeNil())
		expectedDeployment := appsv1.Deployment{}
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		err = yaml.Unmarshal(expectedJSON, &expectedDeployment)
		Expect(err).To(BeNil())

		cs.SetCustomSchedulerExtensionAsDesired(deployment, devConfig)

		Expect(*deployment).To(Equal(expectedDeployment))
	})
})
