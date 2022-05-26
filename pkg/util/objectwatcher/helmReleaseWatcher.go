package objectwatcher

import (
	"errors"

	v2 "github.com/fluxcd/helm-controller/api/v2beta1"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/runner"
)

var _ HelmReleaseWatcher = &helmReleaseWatcherImpl{}

// HelmReleaseWatcher manages operations for helm release dispatched to member clusters.
type HelmReleaseWatcher interface {
	InstallRelease(clusterName string, hr v2.HelmRelease, chart *chart.Chart, values chartutil.Values) (*release.Release, error)
	UpgradeRelease(clusterName string, hr v2.HelmRelease, chart *chart.Chart, values chartutil.Values) (*release.Release, error)
	TestRelease(clusterName string, hr v2.HelmRelease) (*release.Release, error)
	RollbackRelease(clusterName string, hr v2.HelmRelease) error
	UninstallRelease(clusterName string, hr v2.HelmRelease) error
	ObserveLastRelease(clusterName string, hr v2.HelmRelease) (*release.Release, error)
}

// ClientConfigSetFunc is used to generate client config set of member cluster
type ClientConfigSetFunc func(c string, client client.Client) (*rest.Config, error)

type helmReleaseWatcherImpl struct {
	KubeClientSet              client.Client
	ClusterClientConfigSetFunc ClientConfigSetFunc
}

// NewHelmReleaseWatcher returns an instance of HelmReleaseWatcher
func NewHelmReleaseWatcher(kubeClientSet client.Client, clusterClientConfigSetFunc ClientConfigSetFunc) HelmReleaseWatcher {
	return &helmReleaseWatcherImpl{
		KubeClientSet:              kubeClientSet,
		ClusterClientConfigSetFunc: clusterClientConfigSetFunc,
	}
}

func (h *helmReleaseWatcherImpl) InstallRelease(clusterName string, hr v2.HelmRelease, chart *chart.Chart, values chartutil.Values) (*release.Release, error) {
	restConfig, err := h.ClusterClientConfigSetFunc(clusterName, h.KubeClientSet)
	if err != nil {
		return nil, err
	}

	getter := util.NewClusterRESTClientGetter(*restConfig, hr.GetReleaseNamespace())
	runner, err := runner.NewRunner(&getter, hr.GetStorageNamespace())
	if err != nil {
		return nil, err
	}
	rel, err := runner.Install(hr, chart, values)
	return rel, err
}

func (h *helmReleaseWatcherImpl) UpgradeRelease(clusterName string, hr v2.HelmRelease, chart *chart.Chart, values chartutil.Values) (*release.Release, error) {
	restConfig, err := h.ClusterClientConfigSetFunc(clusterName, h.KubeClientSet)
	if err != nil {
		return nil, err
	}

	getter := util.NewClusterRESTClientGetter(*restConfig, hr.GetReleaseNamespace())
	runner, err := runner.NewRunner(&getter, hr.GetStorageNamespace())
	if err != nil {
		return nil, err
	}
	rel, err := runner.Upgrade(hr, chart, values)
	return rel, err
}

func (h *helmReleaseWatcherImpl) TestRelease(clusterName string, hr v2.HelmRelease) (*release.Release, error) {
	restConfig, err := h.ClusterClientConfigSetFunc(clusterName, h.KubeClientSet)
	if err != nil {
		return nil, err
	}

	getter := util.NewClusterRESTClientGetter(*restConfig, hr.GetReleaseNamespace())
	runner, err := runner.NewRunner(&getter, hr.GetStorageNamespace())
	if err != nil {
		return nil, err
	}
	rel, err := runner.Test(hr)
	return rel, err
}

func (h *helmReleaseWatcherImpl) RollbackRelease(clusterName string, hr v2.HelmRelease) error {
	restConfig, err := h.ClusterClientConfigSetFunc(clusterName, h.KubeClientSet)
	if err != nil {
		return err
	}

	getter := util.NewClusterRESTClientGetter(*restConfig, hr.GetReleaseNamespace())
	runner, err := runner.NewRunner(&getter, hr.GetStorageNamespace())
	if err != nil {
		return err
	}
	return runner.Rollback(hr)
}

func (h *helmReleaseWatcherImpl) UninstallRelease(clusterName string, hr v2.HelmRelease) error {
	restConfig, err := h.ClusterClientConfigSetFunc(clusterName, h.KubeClientSet)
	if err != nil {
		return err
	}

	getter := util.NewClusterRESTClientGetter(*restConfig, hr.GetReleaseNamespace())
	runner, err := runner.NewRunner(&getter, hr.GetStorageNamespace())
	if err != nil {
		return err
	}
	if uninstallErr := runner.Uninstall(hr); uninstallErr != nil && !errors.Is(uninstallErr, driver.ErrReleaseNotFound) {
		return uninstallErr
	}
	return nil
}

func (h *helmReleaseWatcherImpl) ObserveLastRelease(clusterName string, hr v2.HelmRelease) (*release.Release, error) {
	restConfig, err := h.ClusterClientConfigSetFunc(clusterName, h.KubeClientSet)
	if err != nil {
		return nil, err
	}

	getter := util.NewClusterRESTClientGetter(*restConfig, hr.GetReleaseNamespace())
	runner, err := runner.NewRunner(&getter, hr.GetStorageNamespace())
	if err != nil {
		return nil, err
	}
	rel, err := runner.ObserveLastRelease(hr)
	if err != nil && errors.Is(err, driver.ErrReleaseNotFound) {
		err = nil
	}
	return rel, err
}
