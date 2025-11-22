/*
Copyright 2025 The Karmada Authors.

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

package metrics

import (
	"fmt"
	"strings"
	"testing"

	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	k8stestutil "k8s.io/component-base/metrics/testutil"
)

func TestCountCreateResourceToCluster(t *testing.T) {
	createResourceWhenSyncWork.Reset()

	cluster := "member-1"
	apiVersion := "v1"
	kind := "Pod"

	CountCreateResourceToCluster(nil, apiVersion, kind, cluster, true)
	CountCreateResourceToCluster(fmt.Errorf("boom"), apiVersion, kind, cluster, false)

	want := `
# HELP create_resource_to_cluster Number of creation operations against a target member cluster. The 'result' label indicates outcome ('success' or 'error'), 'recreate' indicates whether the operation is recreated (true/false). Labels 'apiversion', 'kind', and 'cluster' specify the resource type, API version, and target cluster respectively. [Label deprecation: cluster deprecated in 1.16; use member_cluster. Removal planned 1.18.]
# TYPE create_resource_to_cluster counter
create_resource_to_cluster{apiversion="v1",cluster="member-1",kind="Pod",member_cluster="member-1",recreate="false",result="error"} 1
create_resource_to_cluster{apiversion="v1",cluster="member-1",kind="Pod",member_cluster="member-1",recreate="true",result="success"} 1
`
	if err := promtestutil.CollectAndCompare(createResourceWhenSyncWork, strings.NewReader(want), createResourceToCluster); err != nil {
		t.Fatalf("unexpected collecting result:\n%s", err)
	}
}

func TestCountUpdateResourceToCluster(t *testing.T) {
	updateResourceWhenSyncWork.Reset()

	cluster := "member-2"
	apiVersion := "apps/v1"
	kind := "Deployment"

	CountUpdateResourceToCluster(nil, apiVersion, kind, cluster, "updated")

	want := `
# HELP update_resource_to_cluster Number of updating operation of the resource to a target member cluster. By the result, 'error' means a resource updated failed. Otherwise 'success'. Cluster means the target member cluster. operationResult means the result of the update operation. [Label deprecation: cluster deprecated in 1.16; use member_cluster. Removal planned 1.18.]
# TYPE update_resource_to_cluster counter
update_resource_to_cluster{apiversion="apps/v1",cluster="member-2",kind="Deployment",member_cluster="member-2",operationResult="updated",result="success"} 1
`
	if err := promtestutil.CollectAndCompare(updateResourceWhenSyncWork, strings.NewReader(want), updateResourceToCluster); err != nil {
		t.Fatalf("unexpected collecting result:\n%s", err)
	}
}

func TestCountDeleteResourceFromCluster(t *testing.T) {
	deleteResourceWhenSyncWork.Reset()

	cluster := "member-3"
	apiVersion := "batch/v1"
	kind := "Job"

	// Record an error deletion
	CountDeleteResourceFromCluster(fmt.Errorf("nope"), apiVersion, kind, cluster)

	want := `
# HELP delete_resource_from_cluster Number of deletion operations against a target member cluster. The 'result' label indicates outcome ('success' or 'error'). Labels 'apiversion', 'kind', and 'cluster' specify the resource's API version, type, and source cluster respectively. [Label deprecation: cluster deprecated in 1.16; use member_cluster. Removal planned 1.18.]
# TYPE delete_resource_from_cluster counter
delete_resource_from_cluster{apiversion="batch/v1",cluster="member-3",kind="Job",member_cluster="member-3",result="error"} 1
`
	if err := promtestutil.CollectAndCompare(deleteResourceWhenSyncWork, strings.NewReader(want), deleteResourceFromCluster); err != nil {
		t.Fatalf("unexpected collecting result:\n%s", err)
	}
}

func TestCountPolicyPreemption(t *testing.T) {
	policyPreemptionCounter.Reset()

	// One success, one error
	CountPolicyPreemption(nil)
	CountPolicyPreemption(fmt.Errorf("preempt failed"))

	want := `
# HELP policy_preemption_total Number of preemption for the resource template. By the result, 'error' means a resource template failed to be preempted by other propagation policies. Otherwise 'success'.
# TYPE policy_preemption_total counter
policy_preemption_total{result="error"} 1
policy_preemption_total{result="success"} 1
`
	if err := k8stestutil.CollectAndCompare(policyPreemptionCounter, strings.NewReader(want), policyPreemptionMetricsName); err != nil {
		t.Fatalf("unexpected collecting result:\n%s", err)
	}
}
