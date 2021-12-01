package v1alpha1

import (
	"encoding/json"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/klog/v2"
)

const (
	KindStatefulSet = "StatefulSet"
	KindDeployment  = "Deployment"
)

// AttributesImpl implements k8s.io/apiserver/pkg/admission.Admission
type AttributesImpl struct {
	*admissionv1.AdmissionRequest
}

// GetName returns object name
func (a *AttributesImpl) GetName() string {
	return a.Name
}

// GetNamespace returns object namespace
func (a *AttributesImpl) GetNamespace() string {
	return a.Namespace
}

// GetResource returns object gvr
func (a *AttributesImpl) GetResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    a.Resource.Group,
		Version:  a.Resource.Version,
		Resource: a.Resource.Resource,
	}
}

// GetSubresource returns subresource of attribute
func (a *AttributesImpl) GetSubresource() string {
	return a.SubResource
}

// GetOperation retusn operation
func (a *AttributesImpl) GetOperation() admission.Operation {
	switch a.Operation {
	case admissionv1.Create:
		return admission.Create
	case admissionv1.Update:
		return admission.Update
	case admissionv1.Delete:
		return admission.Delete
	case admissionv1.Connect:
		return admission.Connect
	default:
		return admission.Operation("UNKNOWN")
	}
}

// GetOperationOptions returns operation operations
func (a *AttributesImpl) GetOperationOptions() runtime.Object {
	return a.Options.Object
}

// IsDryRun returns dry-run
func (a *AttributesImpl) IsDryRun() bool {
	if a.DryRun == nil {
		return false
	}
	return *a.DryRun
}

// GetObject returns object
func (a *AttributesImpl) GetObject() runtime.Object {
	klog.V(4).Infof("get Object %s, %s/%s", a.Kind, a.Namespace, a.Name)
	obj, err := unmarshalObject(a.Kind.Kind, a.Object.Raw)
	if err != nil {
		klog.Errorf("unmarshal object failed, kind: %s, %s/%s: %v", a.Kind, a.Namespace, a.Name, err)
		return nil
	}
	return obj
}

// GetOldObject returns old object
func (a *AttributesImpl) GetOldObject() runtime.Object {
	klog.V(4).Infof("get old Object %s, %s/%s", a.Kind, a.Namespace, a.Name)
	obj, err := unmarshalObject(a.Kind.Kind, a.OldObject.Raw)
	if err != nil {
		klog.Errorf("unmarshal old object failed, kind: %s, %s/%s: %v", a.Kind, a.Namespace, a.Name, err)
		return nil
	}
	return obj
}

func unmarshalObject(kind string, raw []byte) (runtime.Object, error) {
	var obj runtime.Object

	switch kind {
	case "ResourceBinding":
		obj = &workv1alpha2.ResourceBinding{}
	case "Service":
		obj = &corev1.Service{}
	case "PersistentVolumeClaim":
		obj = &corev1.PersistentVolumeClaim{}
	}
	if err := json.Unmarshal(raw, obj); err != nil {
		klog.Errorf("unmarshal object failed: %v", err)
		return nil, err
	}
	return obj, nil
}

// GetKind returns object kind
func (a *AttributesImpl) GetKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   a.Kind.Group,
		Version: a.Kind.Version,
		Kind:    a.Kind.Kind,
	}
}

// GetUserInfo returns user info
func (a *AttributesImpl) GetUserInfo() user.Info {
	return ConverUserInfo(a.UserInfo)
}

// AddAnnotations adds given annotation
func (a *AttributesImpl) AddAnnotation(key, value string) error {
	return nil
}

// AddAnnotationWithLevel adds annotation with level
func (a *AttributesImpl) AddAnnotationWithLevel(key, value string, level audit.Level) error {
	return nil
}

// GetReinvocationContext returns reinvocation context
func (a *AttributesImpl) GetReinvocationContext() admission.ReinvocationContext {
	return nil
}

// ConvertAdmissionRequestToAttributes converts admission request in admission webhook to Attributes
func ConvertAdmissionRequestToAttributes(ar *admissionv1.AdmissionRequest) admission.Attributes {
	return &AttributesImpl{AdmissionRequest: ar}
}

// UserInfoImpl implements user.info
type UserInfoImpl struct {
	authenticationv1.UserInfo
}

// GetName returns user name
func (u UserInfoImpl) GetName() string {
	return u.Username
}

// GetUID returns uid
func (u UserInfoImpl) GetUID() string {
	return u.UID
}

// GetGroups returns groups
func (u UserInfoImpl) GetGroups() []string {
	return u.Groups
}

// GetExtra returns extra info
func (u UserInfoImpl) GetExtra() map[string][]string {
	ret := make(map[string][]string)
	for k, v := range u.Extra {
		ret[k] = v
	}
	return ret
}

// ConverUserInfo converts user info in admission request to user.Info
func ConverUserInfo(authUserInfo authenticationv1.UserInfo) user.Info {
	return UserInfoImpl{UserInfo: authUserInfo}
}
