package status

import (
	"context"
	"encoding/json"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/rest"
	"sync"
	"time"
)

var (
	clientManager     *webhookutil.ClientManager
	clientManagerOnce = new(sync.Once)
)

func createClientManagerInternal() (pcm *webhookutil.ClientManager, err error) {
	var cm webhookutil.ClientManager
	if cm, err = webhookutil.NewClientManager(
		[]schema.GroupVersion{configv1alpha1.SchemeGroupVersion},
		configv1alpha1.AddToScheme,
	); err != nil {
		return
	}
	var resolver webhookutil.AuthenticationInfoResolver
	if resolver, err = webhookutil.NewDefaultAuthenticationInfoResolver(""); err != nil {
		return
	}
	pcm = &cm
	pcm.SetAuthenticationInfoResolver(resolver)
	pcm.SetServiceResolver(webhookutil.NewDefaultServiceResolver())
	return
}

func createClientManager() (pcm *webhookutil.ClientManager, err error) {
	clientManagerOnce.Do(func() {
		clientManager, err = createClientManagerInternal()
	})
	pcm = clientManager
	return
}

func parseClientConfiguration(cluster *clusterv1alpha1.Cluster) (config *admissionregistrationv1.WebhookClientConfig, err error) {
	if annotations := cluster.Annotations; annotations == nil {
		return
	} else if value, specified := annotations[externalHealthCheckAnnotationKey]; specified {
		var cfg admissionregistrationv1.WebhookClientConfig
		if err = json.Unmarshal([]byte(value), &cfg); err != nil {
			return
		}
		config = &cfg
	}
	return
}

func hookClientConfigForWebhook(config *admissionregistrationv1.WebhookClientConfig) webhookutil.ClientConfig {
	clientConfig := webhookutil.ClientConfig{Name: "healthz", CABundle: config.CABundle}
	if config.URL != nil {
		clientConfig.URL = *config.URL
	}
	if config.Service != nil {
		clientConfig.Service = &webhookutil.ClientConfigService{
			Name:      config.Service.Name,
			Namespace: config.Service.Namespace,
		}
		if config.Service.Port != nil {
			clientConfig.Service.Port = *config.Service.Port
		} else {
			clientConfig.Service.Port = 443
		}
		if config.Service.Path != nil {
			clientConfig.Service.Path = *config.Service.Path
		}
	}
	return clientConfig
}

func tryExternalProbe(cluster *clusterv1alpha1.Cluster) (pass bool, err error) {
	var config *admissionregistrationv1.WebhookClientConfig
	if config, err = parseClientConfiguration(cluster); err != nil {
		return
	}
	if config == nil {
		pass = true
		return
	}
	var pcm *webhookutil.ClientManager
	if pcm, err = createClientManager(); err != nil {
		return
	}
	var client *rest.RESTClient
	if client, err = pcm.HookClient(hookClientConfigForWebhook(config)); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := client.Get().Do(ctx)
	if e := request.Error(); e != nil {
		if _, ok := e.(*errors.StatusError); !ok {
			err = e
			return
		}
	}
	var status int
	request.StatusCode(&status)
	pass = status/100 == 2
	return
}
