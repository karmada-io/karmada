package framework

import (
	"context"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Proxy connects to a backend
type Proxy interface {
	// Connect returns a http.Handler to connect to backend.
	Connect(ctx context.Context, request ProxyRequest) (http.Handler, error)
}

// Plugin for the proxy plugin framework
type Plugin interface {
	Proxy

	/*
	 * Order should return a constant int value. Smaller int value means higher priority.
	 *
	 * # Why order matters?
	 *
	 * The proxy plugin framework uses a variant of "Chain of Responsibility Pattern".
	 * There will be only one plugin selected. Smaller order value means this plugin has the chance to
	 * handle the request first. Only if the preceding plugin decide to not process the request,
	 * the next plugin can have the chance to process it.
	 *
	 * # Pattern Language Explanation
	 *
	 * "Chain of Responsibility Pattern" From "Design Patterns - Elements of Reusable Object-Oriented Software"
	 *
	 *     Avoid coupling the sender of a request to its receiver by giving more than one object a chance
	 *     to handle the request. Chain the receiving objects and pass the request along the chain until
	 *     an object handles it.
	 *
	 * "Pipes and Filters Pattern" From "Pattern-Oriented Software Architecture Vol.1"
	 *
	 *     The Pipes and Filters architectural pattern provides a structure for systems that process
	 *     a stream of data. Each processing step is encapsulated in a filter component. Data is passed
	 *     through pipes between adjacent filters. Recombining filters allows you to build families of
	 *     related systems.
	 *
	 * Note the difference between "Chain of Responsibility Pattern" and "Pipes and Filters Pattern".
	 * Some other plugin frameworks in kubernetes use "Pipes and Filters Pattern",
	 * they will run multiple filters all together, so the order may not be as important as in
	 * "Chain of Responsibility Pattern"
	 */
	Order() int

	// SupportRequest returns true if this plugin support the request, false if not support.
	// If this method return false, the request will skip this plugin.
	SupportRequest(request ProxyRequest) bool
}

// ProxyRequest holds parameter for Proxy.Connect()
type ProxyRequest struct {
	RequestInfo          *request.RequestInfo
	GroupVersionResource schema.GroupVersionResource
	ProxyPath            string

	Responder rest.Responder

	HTTPReq *http.Request
}
