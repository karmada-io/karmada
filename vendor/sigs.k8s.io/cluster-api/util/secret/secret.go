/*
Copyright 2019 The Kubernetes Authors.

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

package secret

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Get retrieves the specified Secret (if any) from the given
// cluster name and namespace.
func Get(ctx context.Context, c client.Reader, cluster client.ObjectKey, purpose Purpose) (*corev1.Secret, error) {
	return GetFromNamespacedName(ctx, c, cluster, purpose)
}

// GetFromNamespacedName retrieves the specified Secret (if any) from the given
// cluster name and namespace.
func GetFromNamespacedName(ctx context.Context, c client.Reader, clusterName client.ObjectKey, purpose Purpose) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: clusterName.Namespace,
		Name:      Name(clusterName.Name, purpose),
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

// Name returns the name of the secret for a cluster.
func Name(cluster string, suffix Purpose) string {
	return fmt.Sprintf("%s-%s", cluster, suffix)
}

// ParseSecretName return the cluster name and the suffix Purpose in name is a valid cluster secret,
// otherwise it return error.
func ParseSecretName(name string) (string, Purpose, error) {
	separatorPos := strings.LastIndex(name, "-")
	if separatorPos == -1 {
		return "", "", errors.Errorf("%q is not a valid cluster secret name. The purpose suffix is missing", name)
	}
	clusterName := name[:separatorPos]
	purposeSuffix := Purpose(name[separatorPos+1:])
	for _, purpose := range allSecretPurposes {
		if purpose == purposeSuffix {
			return clusterName, purposeSuffix, nil
		}
	}
	return "", "", errors.Errorf("%q is not a valid cluster secret name. Invalid purpose suffix", name)
}
