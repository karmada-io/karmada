package util

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

type sigMultiCluster struct {
	Name          string
	Status        string
	LatestVersion string
	Comment       string
}

func (smc sigMultiCluster) String() string {
	return fmt.Sprintf("%s/%s/%s", smc.Name, smc.Status, smc.LatestVersion)
}

var (
	kubefed = sigMultiCluster{
		Name:          "kubefed",
		Status:        "archived",
		LatestVersion: "v0.9.2",
	}
	karmada = sigMultiCluster{
		Name:          "karmada",
		Status:        "active",
		LatestVersion: "v1.6.0",
	}
	virtualKubelet = sigMultiCluster{
		Name:          "virtual-kubelet",
		Status:        "active",
		LatestVersion: "v1.10.0",
		Comment:       "However, it should be noted that VK is explicitly not intended to be an alternative to Kubernetes federation.",
	}
)

func TestDiffKey(t *testing.T) {
	t.Run("all old remove, all new added", func(t *testing.T) {
		previous := map[string]sigMultiCluster{
			"kubefed": kubefed,
		}
		current := map[string]string{
			"karmada": "v1.6.0",
		}
		added, removed := DiffKey(previous, current)
		if len(added) != 1 || added[0] != "karmada" {
			t.Errorf("added = %v, want []string{\"karmada\"}", added)
		}
		if len(removed) != 1 || removed[0] != "kubefed" {
			t.Errorf("removed = %v, want []string{\"kubefed\"}", removed)
		}
	})
	t.Run("no old, all new added", func(t *testing.T) {
		previous := map[string]sigMultiCluster{}
		current := map[string]string{
			"karmada": "v1.6.0",
		}
		added, removed := DiffKey(previous, current)
		if len(added) != 1 || added[0] != "karmada" {
			t.Errorf("added = %v, want []string{\"karmada\"}", added)
		}
		if len(removed) != 0 {
			t.Errorf("removed = %v, want nil", removed)
		}
	})
	t.Run("removed, added and remained", func(t *testing.T) {
		previous := map[string]sigMultiCluster{
			"kubefed":         kubefed,
			"virtual-kubelet": virtualKubelet,
		}
		current := map[string]string{
			"karmada":         "v1.6.0",
			"virtual-kubelet": virtualKubelet.Comment,
		}
		added, removed := DiffKey(previous, current)
		if len(added) != 1 || added[0] != "karmada" {
			t.Errorf("added = %v, want []string{\"karmada\"}", added)
		}
		if len(removed) != 1 || removed[0] != "kubefed" {
			t.Errorf("removed = %v, want []string{\"kubefed\"}", removed)
		}
	})
}

func TestStringerJoin(t *testing.T) {
	mcs := []sigMultiCluster{kubefed, karmada}
	got := StringerJoin(mcs, "; ")
	expect := "kubefed/archived/v0.9.2; karmada/active/v1.6.0"
	if got != expect {
		t.Errorf("got = %s, want %s", got, expect)
	}
}

func TestKeys(t *testing.T) {
	mcs := map[string]sigMultiCluster{
		kubefed.Name: kubefed,
		karmada.Name: karmada,
	}
	got := Keys(mcs)
	expect := []string{kubefed.Name, karmada.Name}
	sort.Strings(got)
	sort.Strings(expect)
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("got = %v, want %v", got, expect)
	}
}
