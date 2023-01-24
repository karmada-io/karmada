package kubernetes

import (
	"testing"
)

func TestCommandInitOption_etcdServers(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	if got := cmdOpt.etcdServers(); got == "" {
		t.Errorf("CommandInitOption.etcdServers() = %v, want none empty", got)
	}
}

func TestCommandInitOption_karmadaAPIServerContainerCommand(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	flags := cmdOpt.karmadaAPIServerContainerCommand()
	if len(flags) == 0 {
		t.Errorf("CommandInitOption.karmadaAPIServerContainerCommand() returns empty")
	}
}

func TestCommandInitOption_makeKarmadaAPIServerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaAPIServerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaAPIServerDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaKubeControllerManagerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaKubeControllerManagerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaKubeControllerManagerDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaSchedulerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaSchedulerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaSchedulerDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaControllerManagerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaControllerManagerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaControllerManagerDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaWebhookDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaWebhookDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaWebhookDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaAggregatedAPIServerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaAggregatedAPIServerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaAggregatedAPIServerDeployment() returns nil")
	}
}
