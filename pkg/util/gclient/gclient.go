/*
Copyright 2021 The Karmada Authors.

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

package gclient

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// aggregatedScheme aggregates Kubernetes and extended schemes.
var aggregatedScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(scheme.AddToScheme(aggregatedScheme))            // add Kubernetes schemes
	utilruntime.Must(clusterv1alpha1.Install(aggregatedScheme))       // add cluster schemes
	utilruntime.Must(configv1alpha1.Install(aggregatedScheme))        // add config v1alpha1 schemes
	utilruntime.Must(networkingv1alpha1.Install(aggregatedScheme))    // add network v1alpha1 schemes
	utilruntime.Must(policyv1alpha1.Install(aggregatedScheme))        // add propagation schemes
	utilruntime.Must(workv1alpha1.Install(aggregatedScheme))          // add work v1alpha1 schemes
	utilruntime.Must(workv1alpha2.Install(aggregatedScheme))          // add work v1alpha2 schemes
	utilruntime.Must(searchv1alpha1.Install(aggregatedScheme))        // add search v1alpha1 schemes
	utilruntime.Must(mcsv1alpha1.Install(aggregatedScheme))           // add mcs-api schemes
	utilruntime.Must(autoscalingv1alpha1.Install(aggregatedScheme))   // add autoscaling v1alpha1 schemes
	utilruntime.Must(remedyv1alpha1.Install(aggregatedScheme))        // add remedy v1alpha1 schemes
	utilruntime.Must(appsv1alpha1.Install(aggregatedScheme))          // add apps v1alpha1 schemes
	utilruntime.Must(clusterapiv1beta1.AddToScheme(aggregatedScheme)) // add cluster-api v1beta1 schemes
}

// NewSchema returns a singleton schema set which aggregated Kubernetes's schemes and extended schemes.
func NewSchema() *runtime.Scheme {
	return aggregatedScheme
}

// NewForConfig creates a new client for the given config.
func NewForConfig(config *rest.Config) (client.Client, error) {
	return client.New(config, client.Options{
		Scheme: aggregatedScheme,
	})
}

// NewForConfigOrDie creates a new client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(config *rest.Config) client.Client {
	c, err := NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return c
}
