package karmadaquota

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	"github.com/karmada-io/karmada/pkg/quota/v1alpha1/install"
)

// NewKarmadaQuotaWebhook creates new WebhookServer
func NewKarmadaQuotaWebhook(karmadaClient karmadaclientset.Interface) *QuotaAdmission {
	informerFactory := informerfactory.NewSharedInformerFactory(karmadaClient, 5*time.Minute)
	quota, err := NewKarmadaQuota(nil, 5, wait.NeverStop)
	if err != nil {
		klog.Fatalf("create QuotaAdmission failed: %v", err)
	}
	quota.SetQuotaConfiguration(install.NewQuotaConfigurationForAdmission())
	quota.SetExternalKubeClientSet(karmadaClient)
	quota.SetExternalKubeInformerFactory(informerFactory)
	if err := quota.ValidateInitialization(); err != nil {
		klog.Fatalf("validate QuotaAdmission failed: %v", err)
	}

	informerFactory.Start(wait.NeverStop)
	return quota
}
