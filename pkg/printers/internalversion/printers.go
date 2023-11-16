/*
Copyright 2021 The Karmada Authors.

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

package internalversion

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	"github.com/karmada-io/karmada/pkg/printers"
)

// AddHandlers adds print handlers for default Karmada types dealing with internal versions.
func AddHandlers(h printers.PrintHandler) {
	clusterColumnDefinitions := []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "Version", Type: "string", Description: "KubernetesVersion represents version of the member cluster."},
		{Name: "Mode", Type: "string", Description: "SyncMode describes how a cluster sync resources from karmada control plane."},
		{Name: "Ready", Type: "string", Description: "The aggregate readiness state of this cluster for accepting workloads."},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
		{Name: "APIEndpoint", Type: "string", Priority: 1, Description: "The API endpoint of the member cluster."},
	}
	// ignore errors because we enable errcheck golangci-lint.
	_ = h.TableHandler(clusterColumnDefinitions, printClusterList)
	_ = h.TableHandler(clusterColumnDefinitions, printCluster)
}

func printClusterList(clusterList *clusterapis.ClusterList, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	rows := make([]metav1.TableRow, 0, len(clusterList.Items))
	for i := range clusterList.Items {
		r, err := printCluster(&clusterList.Items[i], options)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r...)
	}
	return rows, nil
}

func printCluster(cluster *clusterapis.Cluster, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	ready := "Unknown"
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == clusterapis.ClusterConditionReady {
			ready = string(condition.Status)
			break
		}
	}

	row := metav1.TableRow{
		Object: runtime.RawExtension{Object: cluster},
	}
	row.Cells = append(
		row.Cells,
		cluster.Name,
		cluster.Status.KubernetesVersion,
		cluster.Spec.SyncMode,
		ready,
		translateTimestampSince(cluster.CreationTimestamp))
	if options.Wide {
		row.Cells = append(row.Cells, cluster.Spec.APIEndpoint)
	}
	return []metav1.TableRow{row}, nil
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}
