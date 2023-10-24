package config

import (
	"fmt"
	"strings"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/version"
	"k8s.io/klog/v2"
)

var (
	imageRepositories = map[string]string{
		"global": "registry.k8s.io",
		"cn":     "registry.cn-hangzhou.aliyuncs.com/google_containers",
	}

	DefaultEtcdImage = "etcd:3.5.3-0"

	// DefaultInitImage etcd init container image
	DefaultInitImage string
	// DefaultKarmadaSchedulerImage Karmada scheduler image
	DefaultKarmadaSchedulerImage string
	// DefaultKarmadaControllerManagerImage Karmada controller manager image
	DefaultKarmadaControllerManagerImage string
	// DefualtKarmadaWebhookImage Karmada webhook image
	DefualtKarmadaWebhookImage string
	// DefaultKarmadaAggregatedAPIServerImage Karmada aggregated apiserver image
	DefaultKarmadaAggregatedAPIServerImage string

	karmadaRelease string
)

func init() {
	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}
	karmadaRelease = releaseVer.ReleaseVersion()

	DefaultInitImage = "docker.io/alpine:3.15.1"
	DefaultKarmadaSchedulerImage = fmt.Sprintf("docker.io/karmada/karmada-scheduler:%s", karmadaRelease)
	DefaultKarmadaControllerManagerImage = fmt.Sprintf("docker.io/karmada/karmada-controller-manager:%s", karmadaRelease)
	DefualtKarmadaWebhookImage = fmt.Sprintf("docker.io/karmada/karmada-webhook:%s", karmadaRelease)
	DefaultKarmadaAggregatedAPIServerImage = fmt.Sprintf("docker.io/karmada/karmada-aggregated-apiserver:%s", karmadaRelease)
}

func GetKarmadaRelease() string {
	return karmadaRelease
}

func GetImageRepositories() map[string]string {
	return imageRepositories
}

// get kube components registry
func KubeRegistry(kubeImageRegistry, imageRegistry, kubeImageMirrorCountry string) string {
	registry := kubeImageRegistry

	if registry != "" {
		return registry
	}

	mirrorCountry := strings.ToLower(kubeImageMirrorCountry)

	if mirrorCountry != "" {
		value, ok := imageRepositories[mirrorCountry]
		if ok {
			return value
		}
	}

	if imageRegistry != "" {
		return imageRegistry
	}
	return imageRepositories["global"]
}

func GetKubernetesImage(image string, opt ImageOption) string {
	if opt.SpecImage != "" {
		return opt.SpecImage
	}
	return KubeRegistry(opt.KubeImageRegistry, opt.ImageRegistry, opt.KubeImageMirrorCountry) + "/" + image + ":" + opt.Tag
}

func GetKarmadaImage(image string, opt ImageOption) string {
	if opt.ImageRegistry != "" && opt.SpecImage == opt.DefaultImage {
		return opt.ImageRegistry + "/" + util.KarmadaControllerManager + ":" + karmadaRelease
	}
	return opt.SpecImage
}

type ImageOption struct {
	SpecImage              string
	DefaultImage           string
	Tag                    string
	ImageRegistry          string
	KubeImageRegistry      string
	KubeImageMirrorCountry string
}

// get etcd-init image
func GetetcdInitImage(image string, opt ImageOption) string {
	if opt.ImageRegistry != "" && opt.SpecImage == opt.DefaultImage {
		return opt.ImageRegistry + "/" + util.Alpine + ":" + opt.Tag
	}
	return opt.SpecImage
}
