package apiclient

import (
	"context"
	"errors"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crdsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

var errAllocated = errors.New("provided port is already allocated")

// NewCRDsClient is to create a clientset ClientSet
func NewCRDsClient(c *rest.Config) (*crdsclient.Clientset, error) {
	return crdsclient.NewForConfig(c)
}

// NewAPIRegistrationClient is to create an apiregistration ClientSet
func NewAPIRegistrationClient(c *rest.Config) (*aggregator.Clientset, error) {
	return aggregator.NewForConfig(c)
}

// CreateNamespace creates given namespace when the namespace is not existing.
func CreateNamespace(client clientset.Interface, ns *corev1.Namespace) error {
	_, err := client.CoreV1().Namespaces().Get(context.TODO(), ns.GetName(), metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if _, err := client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created namespace", "namespace", ns.GetName())
	return nil
}

// CreateOrUpdateSecret creates a Sercret if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateSecret(client clientset.Interface, secret *corev1.Secret) error {
	_, err := client.CoreV1().Secrets(secret.GetNamespace()).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.CoreV1().Secrets(secret.GetNamespace()).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated secret", "secret", secret.GetName())
	return nil
}

// CreateOrUpdateService creates a Service if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateService(client clientset.Interface, service *corev1.Service) error {
	_, err := client.CoreV1().Services(service.GetNamespace()).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			_, err := client.CoreV1().Services(service.GetNamespace()).Update(context.TODO(), service, metav1.UpdateOptions{})
			return err
		}

		// Ignore if the Service is invalid with this error message:
		// Service "apiserver" is invalid: provided Port is already allocated.
		if apierrors.IsInvalid(err) && strings.Contains(err.Error(), errAllocated.Error()) {
			klog.V(2).ErrorS(err, "failed to create or update serivce", "service", klog.KObj(service))
			return nil
		}

		return err
	}

	klog.V(5).InfoS("Successfully created or updated service", "service", service.GetName())
	return nil
}

// CreateOrUpdateDeployment creates a Deployment if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateDeployment(client clientset.Interface, deployment *appsv1.Deployment) error {
	_, err := client.AppsV1().Deployments(deployment.GetNamespace()).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		_, err := client.AppsV1().Deployments(deployment.GetNamespace()).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated deployment", "deployment", deployment.GetName())
	return nil
}

// CreateOrUpdateMutatingWebhookConfiguration creates a MutatingWebhookConfiguration if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateMutatingWebhookConfiguration(client clientset.Interface, mwc *admissionregistrationv1.MutatingWebhookConfiguration) error {
	_, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), mwc, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		older, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.TODO(), mwc.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		mwc.ResourceVersion = older.ResourceVersion
		_, err = client.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(context.TODO(), mwc, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated mutatingWebhookConfiguration", "mutatingWebhookConfiguration", mwc.GetName())
	return nil
}

// CreateOrUpdateValidatingWebhookConfiguration creates a ValidatingWebhookConfiguration if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateValidatingWebhookConfiguration(client clientset.Interface, vwc *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	_, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), vwc, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		older, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.TODO(), vwc.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		vwc.ResourceVersion = older.ResourceVersion
		_, err = client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(context.TODO(), vwc, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated validatingWebhookConfiguration", "validatingWebhookConfiguration", vwc.GetName())
	return nil
}

// CreateOrUpdateAPIService creates a APIService if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateAPIService(apiRegistrationClient *aggregator.Clientset, apiservice *apiregistrationv1.APIService) error {
	_, err := apiRegistrationClient.ApiregistrationV1().APIServices().Create(context.TODO(), apiservice, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		older, err := apiRegistrationClient.ApiregistrationV1().APIServices().Get(context.TODO(), apiservice.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		apiservice.ResourceVersion = older.ResourceVersion
		_, err = apiRegistrationClient.ApiregistrationV1().APIServices().Update(context.TODO(), apiservice, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).Infof("Successfully created or updated APIService", "APIService", apiservice.Name)
	return nil
}

// CreateCustomResourceDefinitionIfNeed creates a CustomResourceDefinition if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateCustomResourceDefinitionIfNeed(client *crdsclient.Clientset, obj *apiextensionsv1.CustomResourceDefinition) error {
	crdClient := client.ApiextensionsV1().CustomResourceDefinitions()
	if _, err := crdClient.Create(context.TODO(), obj, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		klog.V(5).InfoS("Skip already exist crd", "crd", obj.Name)
		return nil
	}

	klog.V(5).InfoS("Successfully created crd", "crd", obj.Name)
	return nil
}

// PatchCustomResourceDefinition patchs a crd resource.
func PatchCustomResourceDefinition(client *crdsclient.Clientset, name string, data []byte) error {
	crd := client.ApiextensionsV1().CustomResourceDefinitions()
	if _, err := crd.Patch(context.TODO(), name, types.StrategicMergePatchType, data, metav1.PatchOptions{}); err != nil {
		return err
	}

	klog.V(5).InfoS("Successfully patched crd", "crd", name)
	return nil
}

// CreateOrUpdateStatefulSet creates a StatefulSet if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateStatefulSet(client clientset.Interface, statefuleSet *appsv1.StatefulSet) error {
	_, err := client.AppsV1().StatefulSets(statefuleSet.GetNamespace()).Create(context.TODO(), statefuleSet, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		older, err := client.AppsV1().StatefulSets(statefuleSet.GetNamespace()).Get(context.TODO(), statefuleSet.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		statefuleSet.ResourceVersion = older.ResourceVersion
		_, err = client.AppsV1().StatefulSets(statefuleSet.GetNamespace()).Update(context.TODO(), statefuleSet, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	klog.V(5).InfoS("Successfully created or updated statefulset", "statefulset", statefuleSet.GetName)
	return nil
}
