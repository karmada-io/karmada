package app

// TODO API 目前较少，实现逻辑也暂时放在这里

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
)

type JoinRequest struct {
	// Name of the member cluster
	MemberName string `json:"member_name"`
	// kubeconfig of the karmada apiserver
	KarmadaConfig string `json:"karmada_config"`
	// kubeconfig of the member cluster
	MemberConfig string `json:"member_config"`
	// ClusterProvider of the member cluster
	MemberProvider string `json:"member_provider"`
}

func Join(c *gin.Context) {
	var j JoinRequest
	if err := c.BindJSON(&j); err != nil {
		Response(c, http.StatusBadRequest, "invalid request parmas.", "")
		return
	}

	// check karmada kubeconfig exists ==> cluster_name is unique
	memberClusterConfigName := KARMADA_MEMBER_CONFIG_DIR + "/" + j.MemberName + ".config"
	exist := FileExist(memberClusterConfigName)
	if exist {
		Response(c, http.StatusBadRequest, "cluster name is not unique.", "")
		return
	}

	memberClusterConfig, _ := os.Create(memberClusterConfigName)
	defer func() {
		_ = memberClusterConfig.Close()
	}()

	_, _ = memberClusterConfig.Write([]byte(j.MemberConfig))

	opts := karmadactl.CommandJoinOption{
		GlobalCommandOptions: options.GlobalCommandOptions{
			DryRun: false,
		},
		ClusterName:       j.MemberName,
		ClusterKubeConfig: memberClusterConfigName,
		ClusterProvider:   j.MemberProvider,
	}
	karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, KARMADA_KUBE_CONFIG)
	if err != nil {
		Response(c, http.StatusInternalServerError, fmt.Sprintf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err), "")
		return
	}

	memberClusterRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, memberClusterConfigName)
	if err != nil {
		Response(c, http.StatusInternalServerError, fmt.Sprintf("failed to get member cluster rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err), "")
		return
	}

	err = karmadactl.JoinCluster(controlPlaneRestConfig, memberClusterRestConfig, opts)
	if err != nil {
		Response(c, http.StatusInternalServerError, fmt.Sprintf("failed to join cluster. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err), "")
		return
	}
	Response(c, http.StatusOK, "success", "")
}
