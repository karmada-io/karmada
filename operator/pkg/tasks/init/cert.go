/*
Copyright 2023 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewCertTask init a Certs task to generate all of karmada certs
func NewCertTask(karmada *operatorv1alpha1.Karmada) workflow.Task {
	return workflow.Task{
		Name:        "Certs",
		Run:         runCerts,
		Skip:        skipCerts,
		RunSubTasks: true,
		Tasks:       newCertSubTasks(karmada),
	}
}

func runCerts(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("certs task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[certs] Running certs task", "karmada", klog.KObj(data))
	return nil
}

func skipCerts(d workflow.RunData) (bool, error) {
	data, ok := d.(InitData)
	if !ok {
		return false, errors.New("certs task invoked with an invalid data struct")
	}

	secretName := util.KarmadaCertSecretName(data.GetName())
	secret, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return false, nil
	}

	if err := data.LoadCertFromSecret(secret); err != nil {
		return false, err
	}

	klog.V(4).InfoS("[certs] Successfully loaded certs form secret", "secret", secret.Name, "karmada", klog.KObj(data))
	klog.V(2).InfoS("[certs] Skip certs task, found previous certificates in secret", "karmada", klog.KObj(data))
	return true, nil
}

func newCertSubTasks(karmada *operatorv1alpha1.Karmada) []workflow.Task {
	var subTasks []workflow.Task
	caCert := map[string]*certs.CertConfig{}

	for _, cert := range certs.GetDefaultCertList(karmada) {
		var task workflow.Task

		if cert.CAName == "" {
			task = workflow.Task{Name: cert.Name, Run: runCATask(cert)}
			caCert[cert.Name] = cert
		} else {
			task = workflow.Task{Name: cert.Name, Run: runCertTask(cert, caCert[cert.CAName])}
		}

		subTasks = append(subTasks, task)
	}

	return subTasks
}

func runCATask(kc *certs.CertConfig) func(d workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("certs task invoked with an invalid data struct")
		}

		if kc.CAName != "" {
			return fmt.Errorf("this function should only be used for CAs, but cert %s has CA %s", kc.Name, kc.CAName)
		}

		customCertConfig := data.CustomCertificate()
		if kc.Name == constants.CaCertAndKeyName && customCertConfig.APIServerCACert != nil {
			secretRef := customCertConfig.APIServerCACert
			klog.V(4).InfoS("[certs] Loading custom CA certificate", "secret", secretRef.Name, "namespace", secretRef.Namespace)

			certData, keyData, err := loadCACertFromSecret(data.RemoteClient(), secretRef)
			if err != nil {
				return fmt.Errorf("failed to load custom CA certificate: %w", err)
			}

			klog.V(2).InfoS("[certs] Successfully loaded custom CA certificate", "secret", secretRef.Name)

			customKarmadaCert := certs.NewKarmadaCert(kc.Name, kc.CAName, certData, keyData)

			data.AddCert(customKarmadaCert)
			klog.V(2).InfoS("[certs] Successfully added custom CA certificate to cert store", "certName", kc.Name)
			return nil
		}

		klog.V(4).InfoS("[certs] Creating a new certificate authority", "certName", kc.Name)

		cert, err := certs.NewCertificateAuthority(kc)
		if err != nil {
			return err
		}

		klog.V(2).InfoS("[certs] Successfully generated ca certificate", "certName", kc.Name)

		data.AddCert(cert)
		return nil
	}
}

func loadCACertFromSecret(client clientset.Interface, ref *operatorv1alpha1.LocalSecretReference) ([]byte, []byte, error) {
	secret, err := client.CoreV1().Secrets(ref.Namespace).Get(context.TODO(), ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	certData := secret.Data[constants.TLSCertDataKey]
	keyData := secret.Data[constants.TLSPrivateKeyDataKey]

	if len(certData) == 0 || len(keyData) == 0 {
		return nil, nil, fmt.Errorf("secret %s/%s is missing required keys: %s and %s", ref.Namespace, ref.Name, constants.TLSCertDataKey, constants.TLSPrivateKeyDataKey)
	}

	return certData, keyData, nil
}

func runCertTask(cc, caCert *certs.CertConfig) func(d workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return fmt.Errorf("certs task invoked with an invalid data struct")
		}

		if caCert == nil {
			return fmt.Errorf("unexpected empty ca cert for %s", cc.Name)
		}

		if cc.CAName != caCert.Name {
			return fmt.Errorf("mismatched CA name: expected %s but got %s", cc.CAName, caCert.Name)
		}

		if err := mutateCertConfig(data, cc); err != nil {
			return fmt.Errorf("error when mutate cert altNames for %s, err: %w", cc.Name, err)
		}

		caCert := data.GetCert(cc.CAName)
		cert, err := certs.CreateCertAndKeyFilesWithCA(cc, caCert.CertData(), caCert.KeyData())
		if err != nil {
			return err
		}

		data.AddCert(cert)

		klog.V(2).InfoS("[certs] Successfully generated certificate", "certName", cc.Name, "caName", cc.CAName)
		return nil
	}
}

func mutateCertConfig(data InitData, cc *certs.CertConfig) error {
	if cc.AltNamesMutatorFunc != nil {
		err := cc.AltNamesMutatorFunc(&certs.AltNamesMutatorConfig{
			Name:                data.GetName(),
			Namespace:           data.GetNamespace(),
			Components:          data.Components(),
			ControlplaneAddress: data.ControlplaneAddress(),
		}, cc)

		if err != nil {
			return err
		}
	}

	if data.CustomCertificate().LeafCertValidityDays != nil {
		certValidityDuration := time.Hour * 24 * time.Duration(*data.CustomCertificate().LeafCertValidityDays)
		notAfter := time.Now().Add(certValidityDuration).UTC()
		cc.NotAfter = &notAfter
	}

	return nil
}
