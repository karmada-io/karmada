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

package remediation

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
)

func isRemedyWorkOnCluster(remedy *remedyv1alpha1.Remedy, cluster *clusterv1alpha1.Cluster) bool {
	if remedy.Spec.ClusterAffinity == nil {
		return true
	}

	for _, clusterName := range remedy.Spec.ClusterAffinity.ClusterNames {
		if clusterName == cluster.Name {
			return true
		}
	}
	return false
}

func remedyDecisionMatchWithCluster(decisionMatches []remedyv1alpha1.DecisionMatch, conditions []metav1.Condition) bool {
	if decisionMatches == nil {
		return true
	}

	if conditions == nil {
		return false
	}

	for _, decisionMatch := range decisionMatches {
		if decisionMatch.ClusterConditionMatch == nil {
			continue
		}

		conditionType := decisionMatch.ClusterConditionMatch.ConditionType
		findStatusCondition := meta.FindStatusCondition(conditions, string(conditionType))
		if findStatusCondition == nil {
			continue
		}

		status := decisionMatch.ClusterConditionMatch.ConditionStatus
		switch decisionMatch.ClusterConditionMatch.Operator {
		case remedyv1alpha1.ClusterConditionEqual:
			if status == string(findStatusCondition.Status) {
				return true
			}
		case remedyv1alpha1.ClusterConditionNotEqual:
			if status != string(findStatusCondition.Status) {
				return true
			}
		}
	}

	return false
}

func calculateActions(clusterRelatedRemedies []*remedyv1alpha1.Remedy, cluster *clusterv1alpha1.Cluster) []string {
	actionSet := sets.NewString()
	for _, remedy := range clusterRelatedRemedies {
		if remedyDecisionMatchWithCluster(remedy.Spec.DecisionMatches, cluster.Status.Conditions) {
			for _, action := range remedy.Spec.Actions {
				actionSet.Insert(string(action))
			}
		}
	}
	return actionSet.List()
}

func getClusterRelatedRemedies(ctx context.Context, client client.Client, cluster *clusterv1alpha1.Cluster) ([]*remedyv1alpha1.Remedy, error) {
	remedyList := &remedyv1alpha1.RemedyList{}
	if err := client.List(ctx, remedyList); err != nil {
		return nil, err
	}

	var clusterRelatedRemedies []*remedyv1alpha1.Remedy
	for index := range remedyList.Items {
		remedy := remedyList.Items[index]
		if isRemedyWorkOnCluster(&remedy, cluster) {
			clusterRelatedRemedies = append(clusterRelatedRemedies, &remedy)
		}
	}
	return clusterRelatedRemedies, nil
}
