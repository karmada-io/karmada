package runtime

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
)

// frameworkImpl select appropriate plugin to do `Connect()`
type frameworkImpl struct {
	plugins []framework.Plugin
}

// frameworkImpl is actually a Proxy
var _ framework.Proxy = (*frameworkImpl)(nil)

// NewFramework create instance of framework.Proxy with determined order of Plugin.
func NewFramework(plugins []framework.Plugin) framework.Proxy {
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Order() < plugins[j].Order()
	})

	return &frameworkImpl{
		plugins: plugins,
	}
}

// Connect implements Proxy
func (c *frameworkImpl) Connect(ctx context.Context, request framework.ProxyRequest) (http.Handler, error) {
	plugin, err := c.selectPlugin(request)
	if err != nil {
		return nil, err
	}

	return plugin.Connect(ctx, request)
}

// selectPlugin return an appropriate Plugin by query Plugin.SupportRequest in order.
func (c *frameworkImpl) selectPlugin(request framework.ProxyRequest) (framework.Plugin, error) {
	for _, plugin := range c.plugins {
		if plugin.SupportRequest(request) {
			return plugin, nil
		}
	}

	return nil, fmt.Errorf("no plugin found for request: %v %v",
		request.RequestInfo.Verb, request.RequestInfo.Path)
}
