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

package runtime

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	"k8s.io/client-go/informers"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	"github.com/karmada-io/karmada/pkg/estimator/server/metrics"
	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

const (
	estimator = "Estimator"
)

// frameworkImpl implements the Framework interface and is responsible for initializing and running scheduler
// plugins.
type frameworkImpl struct {
	estimateReplicasPlugins []framework.EstimateReplicasPlugin
	informerFactory         informers.SharedInformerFactory
}

var _ framework.Framework = &frameworkImpl{}

type frameworkOptions struct {
	informerFactory informers.SharedInformerFactory
}

// Option for the frameworkImpl.
type Option func(*frameworkOptions)

func defaultFrameworkOptions() frameworkOptions {
	return frameworkOptions{}
}

// WithInformerFactory sets informer factory for the scheduling frameworkImpl.
func WithInformerFactory(informerFactory informers.SharedInformerFactory) Option {
	return func(o *frameworkOptions) {
		o.informerFactory = informerFactory
	}
}

// NewFramework creates a scheduling framework by registry.
func NewFramework(r Registry, opts ...Option) (framework.Framework, error) {
	options := defaultFrameworkOptions()
	for _, opt := range opts {
		opt(&options)
	}
	f := &frameworkImpl{
		informerFactory: options.informerFactory,
	}
	estimateReplicasPluginsList := reflect.ValueOf(&f.estimateReplicasPlugins).Elem()
	estimateReplicasType := estimateReplicasPluginsList.Type().Elem()

	for name, factory := range r {
		p, err := factory(f)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize plugin %q: %w", name, err)
		}
		addPluginToList(p, estimateReplicasType, &estimateReplicasPluginsList)
	}
	return f, nil
}

func addPluginToList(plugin framework.Plugin, pluginType reflect.Type, pluginList *reflect.Value) {
	if reflect.TypeOf(plugin).Implements(pluginType) {
		newPlugins := reflect.Append(*pluginList, reflect.ValueOf(plugin))
		pluginList.Set(newPlugins)
	}
}

// SharedInformerFactory returns a shared informer factory.
func (frw *frameworkImpl) SharedInformerFactory() informers.SharedInformerFactory {
	return frw.informerFactory
}

func (frw *frameworkImpl) RunEstimateReplicasPlugins(ctx context.Context, replicaRequirements *pb.ReplicaRequirements) (int32, error) {
	startTime := time.Now()
	defer func() {
		metrics.FrameworkExtensionPointDuration.WithLabelValues(estimator).Observe(utilmetrics.DurationInSeconds(startTime))
	}()
	var result int32 = math.MaxInt32
	for _, pl := range frw.estimateReplicasPlugins {
		if replica, err := frw.runEstimateReplicasPlugins(ctx, pl, replicaRequirements); err == nil {
			result = func(a, b int32) int32 {
				if a <= b {
					return a
				}
				return b
			}(replica, result)
		}
	}
	return result, nil
}

func (frw *frameworkImpl) runEstimateReplicasPlugins(
	ctx context.Context,
	pl framework.EstimateReplicasPlugin,
	replicaRequirements *pb.ReplicaRequirements,
) (int32, error) {
	startTime := time.Now()
	result, err := pl.Estimate(ctx, replicaRequirements)
	metrics.PluginExecutionDuration.WithLabelValues(pl.Name(), estimator).Observe(utilmetrics.DurationInSeconds(startTime))
	return result, err
}
