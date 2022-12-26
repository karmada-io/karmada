package karmada

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/resource"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	utilresource "github.com/karmada-io/karmada/operator/pkg/util/resource"
)

func (ctrl *Controller) ensureKarmadaCRDs(karmada *operatorv1alpha1.Karmada) error {
	clientConfig, err := ctrl.GenerateClientConfig(karmada)
	if err != nil {
		return err
	}
	// fixme(@carlory): use the correct crds path
	result := utilresource.NewBuilder(clientConfig).
		Unstructured().
		FilenameParam(false, &resource.FilenameOptions{Recursive: false, Filenames: []string{"./pkg/controller/karmada/crds"}}).
		Flatten().Do()
	return result.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		_, err1 := resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, true, info.Object)
		if err1 != nil {
			if !errors.IsAlreadyExists(err1) {
				return err1
			}
		}
		return nil
	})
}
