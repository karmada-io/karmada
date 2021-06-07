package detector

import (
	"reflect"
	"sort"
	"testing"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
)

func TestResourceDetector_GetMatching(t *testing.T) {
	tests := []struct {
		name              string
		waitingObjects    map[keys.ClusterWideKey]struct{}
		resourceSelectors []policyv1alpha1.ResourceSelector
		want              []keys.ClusterWideKey
	}{
		{
			name: "nil resourceSelectors",
			waitingObjects: map[keys.ClusterWideKey]struct{}{
				{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod-a"}:                       {},
				{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "ns-a", Name: "deploy-b"}: {},
			},
			resourceSelectors: nil,
			want: []keys.ClusterWideKey{
				{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod-a"},
				{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "ns-a", Name: "deploy-b"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{
				waitingObjects: tt.waitingObjects,
			}

			got := d.GetMatching(tt.resourceSelectors)
			sort.Slice(tt.want, func(i, j int) bool { return tt.want[i].Name < tt.want[j].Name })
			sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMatching() = %v, want %v", got, tt.want)
			}
		})
	}
}
