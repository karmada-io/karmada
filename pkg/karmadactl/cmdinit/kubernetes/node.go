package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

func (i *CommandInitOption) getKarmadaAPIServerIP() error {
	if i.KarmadaAPIServerAdvertiseAddress != "" {
		i.KarmadaAPIServerIP = append(i.KarmadaAPIServerIP, utils.StringToNetIP(i.KarmadaAPIServerAdvertiseAddress))
		return nil
	}

	nodeClient := i.KubeClientSet.CoreV1().Nodes()
	masterNodes, err := nodeClient.List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
	if err != nil {
		return err
	}
	if len(masterNodes.Items) == 0 {
		masterNodes, err = nodeClient.List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/control-plane"})
		if err != nil {
			return err
		}
	}

	if len(masterNodes.Items) == 0 {
		klog.Warning("The kubernetes cluster does not have a Master role.")
	} else {
		for _, v := range masterNodes.Items {
			i.KarmadaAPIServerIP = append(i.KarmadaAPIServerIP, utils.StringToNetIP(v.Status.Addresses[0].Address))
		}
		return nil
	}

	klog.Info("Randomly select 3 Node IPs in the kubernetes cluster.")
	nodes, err := nodeClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for number := 0; number < 3; number++ {
		if number >= len(nodes.Items) {
			break
		}
		i.KarmadaAPIServerIP = append(i.KarmadaAPIServerIP, utils.StringToNetIP(nodes.Items[number].Status.Addresses[0].Address))
	}

	if len(i.KarmadaAPIServerIP) == 0 {
		return fmt.Errorf("karmada apiserver ip is empty")
	}
	return nil
}

// nodeStatus Check the node status, if it is an unhealthy node, return false.
func nodeStatus(nodeConditions []corev1.NodeCondition) bool {
	for _, v := range nodeConditions {
		switch v.Type {
		case corev1.NodeReady:
			if v.Status != corev1.ConditionTrue {
				return false
			}
		case corev1.NodeMemoryPressure:
			if v.Status != corev1.ConditionFalse {
				return false
			}
		case corev1.NodeDiskPressure:
			if v.Status != corev1.ConditionFalse {
				return false
			}
		case corev1.NodeNetworkUnavailable:
			if v.Status != corev1.ConditionFalse {
				return false
			}
		case corev1.NodePIDPressure:
			if v.Status != corev1.ConditionFalse {
				return false
			}
		}
	}
	return true
}

// AddNodeSelectorLabels When EtcdStorageMode is hostPath, and EtcdNodeSelectorLabels is empty.
// Select a healthy node to add labels, and schedule etcd to that node
func (i *CommandInitOption) AddNodeSelectorLabels() error {
	nodes, err := i.KubeClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	var etcdNodeName string
	var etcdSelectorLabels map[string]string
	for _, v := range nodes.Items {
		if v.Spec.Taints != nil {
			continue
		}

		if nodeStatus(v.Status.Conditions) {
			etcdNodeName = v.Name
			etcdSelectorLabels = v.Labels
			if etcdSelectorLabels == nil {
				etcdSelectorLabels = map[string]string{}
			}
			break
		}
	}

	if etcdSelectorLabels == nil {
		return errors.New("failed to find a healthy node for karmada-etcd")
	}

	etcdSelectorLabels["karmada.io/etcd"] = ""
	patchData := map[string]interface{}{"metadata": map[string]map[string]string{"labels": etcdSelectorLabels}}
	playLoadBytes, _ := json.Marshal(patchData)
	if _, err := i.KubeClientSet.CoreV1().Nodes().Patch(context.TODO(), etcdNodeName, types.StrategicMergePatchType, playLoadBytes, metav1.PatchOptions{}); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) isNodeExist(labels string) bool {
	node, err := i.KubeClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return false
	}

	if len(node.Items) == 0 {
		return false
	}
	klog.Infof("Find the node [ %s ] by label %s", node.Items[0].Name, labels)
	return true
}
