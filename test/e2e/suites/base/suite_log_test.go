/*
Copyright 2024 The Karmada Authors.

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

package base

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const logDir = "/tmp/karmada/objects"

func printAllBindingAndRelatedObjects() {
	filePath := filepath.Join(logDir, testNamespace+".txt")
	err := os.MkdirAll(logDir, 0700)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	logFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	defer logFile.Close()

	rbList, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	for _, rb := range rbList.Items {
		// print resource binding to log file
		printObjectToFile(logFile, &rb)
		printBindingRelatedResourceAndWork(logFile, rb.ObjectMeta, rb.Spec, workv1alpha2.ResourceBindingPermanentIDLabel)
	}

	crbList, err := karmadaClient.WorkV1alpha2().ClusterResourceBindings().List(context.TODO(), metav1.ListOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	for _, crb := range crbList.Items {
		// print cluster resource binding to log file
		printObjectToFile(logFile, &crb)
		printBindingRelatedResourceAndWork(logFile, crb.ObjectMeta, crb.Spec, workv1alpha2.ClusterResourceBindingPermanentIDLabel)
	}

	clusterList, err := karmadaClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	for _, cluster := range clusterList.Items {
		// print cluster object to log file
		printObjectToFile(logFile, &cluster)
	}
}

func printBindingRelatedResourceAndWork(file *os.File, metadata metav1.ObjectMeta, spec workv1alpha2.ResourceBindingSpec, permanentIDLabelKey string) {
	gvr, err := restmapper.GetGroupVersionResource(restMapper, schema.FromAPIVersionAndKind(spec.Resource.APIVersion, spec.Resource.Kind))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	resource, err := dynamicClient.Resource(gvr).Namespace(spec.Resource.Namespace).Get(context.TODO(), spec.Resource.Name, metav1.GetOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// print resource template to log file
	printObjectToFile(file, resource)

	workList, err := karmadaClient.WorkV1alpha1().Works("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			permanentIDLabelKey: metadata.Labels[permanentIDLabelKey],
		}).String(),
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	for _, work := range workList.Items {
		// print work to log file
		printObjectToFile(file, &work)
	}
}

func printObjectToFile(file *os.File, obj metav1.Object) {
	obj.SetManagedFields(nil)

	objBytes, err := json.Marshal(obj)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	objBytes = append(objBytes, '\n', '\n')

	_, err = file.Write(objBytes)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}
