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

package promote

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/runtime"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/klog/v2"
	"k8s.io/kube-aggregator/pkg/apiserver"
	utiltrace "k8s.io/utils/trace"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/webhook/configmanager"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/webhook/request"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	interpreterutil "github.com/karmada-io/karmada/pkg/util/interpreter"
)

// PortForwardProtocolV1Name is the subprotocol used for port forwarding.
const PortForwardProtocolV1Name = "portforward.k8s.io"
const LocalHost = "127.0.0.1"

// customizedInterpreter interpret custom resource with webhook configuration.
type customizedInterpreter struct {
	// hookManager caches all webhook configurations.
	hookManager configmanager.ConfigManager
	// clientManager builds REST clients to talk to webhooks.
	clientManager *webhookutil.ClientManager
}

// newCustomizedInterpreter return a new CustomizedInterpreter.
func newCustomizedInterpreter(informer genericmanager.SingleClusterInformerManager, serviceLister v1.ServiceLister) (*customizedInterpreter, error) {
	cm, err := webhookutil.NewClientManager(
		[]schema.GroupVersion{configv1alpha1.SchemeGroupVersion},
		configv1alpha1.AddToScheme,
	)
	if err != nil {
		return nil, err
	}
	authInfoResolver, err := webhookutil.NewDefaultAuthenticationInfoResolver("")
	if err != nil {
		return nil, err
	}

	cm.SetAuthenticationInfoResolver(authInfoResolver)
	cm.SetServiceResolver(apiserver.NewClusterIPServiceResolver(serviceLister))

	return &customizedInterpreter{
		hookManager:   configmanager.NewExploreConfigManager(informer),
		clientManager: &cm,
	}, nil
}

// HookEnabled tells if any hook exist for specific resource gvk and operation type.
func (e *customizedInterpreter) HookEnabled(objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) bool {
	if !e.hookManager.HasSynced() {
		klog.Errorf("not yet ready to handle request")
		return false
	}

	hook := e.getFirstRelevantHook(objGVK, operation)
	if hook == nil {
		klog.V(4).Infof("Hook interpreter is not enabled for kind %q with operation %q.", objGVK, operation)
	}
	return hook != nil
}

// getDependencies returns the dependencies of give object.
// return matched value to indicate whether there is a matching hook.
func (e *customizedInterpreter) getDependencies(ctx context.Context, attributes *request.Attributes,
	context string, kubeconfigPath string, namespace string, pod string, port int) ([]configv1alpha1.DependentObjectReference, bool, error) {
	if !e.hookManager.HasSynced() {
		return nil, false, fmt.Errorf("not yet ready to handle request")
	}

	hook := e.getFirstRelevantHook(attributes.Object.GroupVersionKind(), attributes.Operation)
	if hook == nil {
		return nil, false, nil
	}

	// Check if the request has already timed out before spawning remote calls
	select {
	case <-ctx.Done():
		// parent context is canceled or timed out, no point in continuing
		err := apierrors.NewTimeoutError("request did not complete within requested timeout", 0)
		return nil, true, err
	default:
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	var response *request.ResponseAttributes
	var callErr error
	wh := newWebhookInterpreterHelper(hook, e.clientManager, attributes)
	go func(hook configmanager.WebhookAccessor) {
		defer wg.Done()
		response, callErr = wh.callHook(ctx, context, kubeconfigPath, namespace, pod, port)
		if callErr != nil {
			klog.Warningf("Failed calling webhook %v: %v", hook.GetUID(), callErr)
			callErr = apierrors.NewInternalError(callErr)
		}
	}(hook)

	wg.Wait()

	if callErr != nil {
		return nil, true, callErr
	}
	if response == nil {
		return nil, true, apierrors.NewInternalError(fmt.Errorf("get nil response from webhook call"))
	}
	return response.Dependencies, true, nil
}

func (e *customizedInterpreter) getFirstRelevantHook(objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) configmanager.WebhookAccessor {
	relevantHooks := make([]configmanager.WebhookAccessor, 0)
	for _, hook := range e.hookManager.HookAccessors() {
		if shouldCallHook(hook, objGVK, operation) {
			relevantHooks = append(relevantHooks, hook)
		}
	}

	if len(relevantHooks) == 0 {
		return nil
	}

	// Sort relevantHooks alphabetically, taking the first element for spawning remote calls
	sort.SliceStable(relevantHooks, func(i, j int) bool {
		return relevantHooks[i].GetUID() < relevantHooks[j].GetUID()
	})
	return relevantHooks[0]
}

func shouldCallHook(hook configmanager.WebhookAccessor, objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) bool {
	for _, rule := range hook.GetRules() {
		matcher := interpreterutil.Matcher{
			ObjGVK:    objGVK,
			Operation: operation,
			Rule:      rule,
		}
		if matcher.Matches() {
			return true
		}
	}
	return false
}

// hookClientConfigForWebhook construct a webhookutil.ClientConfig using an admissionregistrationv1.WebhookClientConfig
// to access v1alpha1.ResourceInterpreterWebhook. This changes the URL to point to the local port.
func hookClientConfigForWebhook(hookName string, config admissionregistrationv1.WebhookClientConfig, address string, localPort string) webhookutil.ClientConfig {
	clientConfig := webhookutil.ClientConfig{Name: hookName, CABundle: config.CABundle}
	var requestPath string
	if config.URL != nil {
		requestPath = path.Base(*config.URL)
	}
	if config.Service != nil {
		if config.Service.Path != nil {
			requestPath = *config.Service.Path
		}
	}
	clientConfig.URL = fmt.Sprintf("https://%s:%s/%s", address, localPort, requestPath)
	return clientConfig
}

// webhookInterpreterHelper is a helper struct for calling webhook interpreter.
type webhookInterpreterHelper struct {
	// webhook configuration
	hook          configmanager.WebhookAccessor
	clientManager *webhookutil.ClientManager
	attributes    *request.Attributes
}

// PortForwarder knows how to listen for local connections and forward them to
// a remote pod via an upgraded HTTP request.
type PortForwarder struct {
	// port forwarding configuration
	dialer        httpstream.Dialer
	streamConn    httpstream.Connection
	Ready         chan struct{}
	requestIDLock sync.Mutex
	requestID     int
	address       string
	localPort     int
	remotePort    int
	listener      io.Closer
	out           io.Writer
	errOut        io.Writer
}

func newWebhookInterpreterHelper(hook configmanager.WebhookAccessor, clientManager *webhookutil.ClientManager, attributes *request.Attributes) *webhookInterpreterHelper {
	return &webhookInterpreterHelper{
		hook:          hook,
		clientManager: clientManager,
		attributes:    attributes,
	}
}

func (wh *webhookInterpreterHelper) callHook(ctx context.Context, context string, kubeconfigPath string, namespace string, pod string, port int) (*request.ResponseAttributes, error) {
	restConfig, err := apiclient.RestConfig(context, kubeconfigPath)
	if err != nil {
		return nil, err
	}

	KubeClientSet, err := apiclient.NewClientSet(restConfig)
	if err != nil {
		return nil, err
	}
	portforwardReq := KubeClientSet.CoreV1().RESTClient().Post().Namespace(namespace).
		Resource("pods").Name(pod).SubResource("portforward")
	ReadyChannel := make(chan struct{})

	res, err := ForwardPorts(ctx, "POST", portforwardReq.URL(), restConfig, ReadyChannel, wh, port)
	if err != nil {
		klog.Fatalln(err)
	}

	return res, nil
}

func ForwardPorts(ctx context.Context, method string, url *url.URL, config *rest.Config, ReadyChannel chan struct{}, wh *webhookInterpreterHelper, port int) (*request.ResponseAttributes, error) {
	IOStreams := struct {
		// In think, os.Stdin
		In io.Reader
		// Out think, os.Stdout
		Out io.Writer
		// ErrOut think, os.Stderr
		ErrOut io.Writer
	}{os.Stdin, os.Stdout, os.Stderr}

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)

	fw, err := NewOnAddresses(dialer, ReadyChannel, IOStreams.Out, IOStreams.ErrOut, port)
	if err != nil {
		return nil, err
	}
	resp, err := fw.ForwardPorts(ctx, wh)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ForwardPorts formats and executes a port forwarding request.
func (pf *PortForwarder) ForwardPorts(ctx context.Context, wh *webhookInterpreterHelper) (*request.ResponseAttributes, error) {
	defer pf.Close()

	var err error
	pf.streamConn, _, err = pf.dialer.Dial(PortForwardProtocolV1Name)
	if err != nil {
		return nil, fmt.Errorf("error upgrading connection: %s", err)
	}
	defer pf.streamConn.Close()

	rep, err := pf.forward(ctx, wh)
	return rep, err
}

// Close stops the listener of PortForwarder.
func (pf *PortForwarder) Close() {
	// stop the listener
	if err := pf.listener.Close(); err != nil {
		runtime.HandleError(fmt.Errorf("error closing listener: %v", err))
	}
}

// NewOnAddresses creates a new PortForwarder with custom listen addresses.
func NewOnAddresses(dialer httpstream.Dialer, readyChan chan struct{}, out, errOut io.Writer, port int) (*PortForwarder, error) {
	return &PortForwarder{
		dialer:     dialer,
		Ready:      readyChan,
		out:        out,
		errOut:     errOut,
		address:    LocalHost,
		localPort:  0, // this port is set by getListener to get the real local port
		remotePort: port,
	}, nil
}

// listenOnPortAndAddress delegates listener creation and waits for new connections
// in the background.
func (pf *PortForwarder) listenOnPortAndAddress() error {
	listener, err := pf.getListener("tcp")
	if err != nil {
		return err
	}
	pf.listener = listener
	go pf.waitForConnection(listener)
	return nil
}

// waitForConnection waits for new connections to listener and handles them in
// the background.
func (pf *PortForwarder) waitForConnection(listener net.Listener) {
	for {
		select {
		case <-pf.streamConn.CloseChan():
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				// TODO consider using something like https://github.com/hydrogen18/stoppableListener?
				if !strings.Contains(strings.ToLower(err.Error()), "use of closed network connection") {
					runtime.HandleError(fmt.Errorf("error accepting connection on port %d: %v", pf.localPort, err))
				}
				return
			}
			go pf.handleConnection(conn)
		}
	}
}

// getListener creates a listener on the interface targeted by the given hostname on the given port with
// the given protocol. protocol is in net.Listen style which basically admits values like tcp, tcp4, tcp6
func (pf *PortForwarder) getListener(protocol string) (net.Listener, error) {
	listener, err := net.Listen(protocol, net.JoinHostPort(pf.address, strconv.Itoa(pf.localPort)))
	if err != nil {
		return nil, fmt.Errorf("unable to create listener: Error %s", err)
	}
	listenerAddress := listener.Addr().String()
	host, localPort, _ := net.SplitHostPort(listenerAddress)
	localPortUInt, err := strconv.ParseUint(localPort, 10, 16)

	if err != nil {
		fmt.Fprintf(pf.out, "Failed to forward from %s:%d -> %d\n", pf.address, localPortUInt, pf.remotePort)
		return nil, fmt.Errorf("error parsing local port: %s from %s (%s)", err, listenerAddress, host)
	}
	pf.localPort = int(uint16(localPortUInt))
	if pf.out != nil {
		fmt.Fprintf(pf.out, "Forwarding from %s -> %d\n", net.JoinHostPort(pf.address, strconv.Itoa(int(localPortUInt))), pf.remotePort)
	}

	return listener, nil
}

// forward dials the remote host specific in req, upgrades the request, starts
// listener for port, and forwards local connections
// to the remote host via streams.
func (pf *PortForwarder) forward(ctx context.Context, wh *webhookInterpreterHelper) (*request.ResponseAttributes, error) {
	var err error

	listenSuccess := false

	err = pf.listenOnPortAndAddress()
	switch {
	case err == nil:
		listenSuccess = true
	default:
		if pf.errOut != nil {
			fmt.Fprintf(pf.errOut, "Unable to listen on port %v: %v\n", pf.localPort, err)
		}
	}

	if !listenSuccess {
		return nil, fmt.Errorf("unable to listen on any of the requested ports: %v", pf.localPort)
	}

	if pf.Ready != nil {
		close(pf.Ready)
	}

	uid, req, err := request.CreateResourceInterpreterContext(wh.hook.GetInterpreterContextVersions(), wh.attributes)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: wh.hook.GetUID(),
			Reason:      fmt.Errorf("could not create ResourceInterpreterContext objects: %w", err),
		}
	}

	trace := utiltrace.New("Call resource interpret webhook",
		utiltrace.Field{Key: "configuration", Value: wh.hook.GetConfigurationName()},
		utiltrace.Field{Key: "webhook", Value: wh.hook.GetName()},
		utiltrace.Field{Key: "kind", Value: wh.attributes.Object.GroupVersionKind()},
		utiltrace.Field{Key: "operation", Value: wh.attributes.Operation},
		utiltrace.Field{Key: "UID", Value: uid})
	defer trace.LogIfLong(500 * time.Millisecond)

	// if the webhook has a specific timeout, wrap the context to apply it
	if wh.hook.GetTimeoutSeconds() != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*wh.hook.GetTimeoutSeconds())*time.Second)
		defer cancel()
	}

	client, err := wh.clientManager.HookClient(hookClientConfigForWebhook(wh.hook.GetName(), wh.hook.GetClientConfig(), pf.address, strconv.Itoa(pf.localPort)))
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: wh.hook.GetUID(),
			Reason:      fmt.Errorf("could not get REST client: %w", err),
		}
	}
	r := client.Post().Body(req)

	// if the context has a deadline, set it as a parameter to inform the backend
	if deadline, ok := ctx.Deadline(); ok {
		if timeout := time.Until(deadline); timeout > 0 {
			// if it's not an even number of seconds, round up to the nearest second
			if truncated := timeout.Truncate(time.Second); truncated != timeout {
				timeout = truncated + time.Second
			}
			r.Timeout(timeout)
		}
	}

	response := &configv1alpha1.ResourceInterpreterContext{}
	err = r.Do(ctx).Into(response)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: wh.hook.GetUID(),
			Reason:      fmt.Errorf("failed to call webhook: %w", err),
		}
	}

	var res *request.ResponseAttributes
	res, err = request.VerifyResourceInterpreterContext(uid, wh.attributes.Operation, response)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: wh.hook.GetUID(),
			Reason:      fmt.Errorf("reveived invalid webhook response: %w", err),
		}
	}

	if !res.Successful {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: wh.hook.GetUID(),
			Reason:      fmt.Errorf("webhook call failed, get status code: %d, msg: %s", res.Status.Code, res.Status.Message),
		}
	}

	return res, nil
}

func (pf *PortForwarder) nextRequestID() int {
	pf.requestIDLock.Lock()
	defer pf.requestIDLock.Unlock()
	id := pf.requestID
	pf.requestID++
	return id
}

// handleConnection copies data between the local connection and the stream to
// the remote server.
func (pf *PortForwarder) handleConnection(conn net.Conn) {
	defer conn.Close()

	requestID := pf.nextRequestID()

	// create error stream
	headers := http.Header{}
	headers.Set(corev1.StreamType, corev1.StreamTypeError)
	headers.Set(corev1.PortHeader, fmt.Sprintf("%d", pf.remotePort))
	headers.Set(corev1.PortForwardRequestIDHeader, strconv.Itoa(requestID))
	errorStream, err := pf.streamConn.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating error stream for port %d : %v", pf.remotePort, err))
		return
	}
	// we're not writing to this stream
	errorStream.Close()
	defer pf.streamConn.RemoveStreams(errorStream)

	errorChan := make(chan error)
	go func() {
		message, err := io.ReadAll(errorStream)
		switch {
		case err != nil:
			errorChan <- fmt.Errorf("error reading from error stream for port %d : %v", pf.remotePort, err)
		case len(message) > 0:
			errorChan <- fmt.Errorf("an error occurred forwarding port %d : %v", pf.remotePort, string(message))
		}
		close(errorChan)
	}()

	// create data stream
	headers.Set(corev1.StreamType, corev1.StreamTypeData)

	dataStream, err := pf.streamConn.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating forwarding stream for port %d : %v", pf.remotePort, err))
		return
	}
	defer pf.streamConn.RemoveStreams(dataStream)

	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	go func() {
		// Copy from the remote side to the local port.
		if _, err := io.Copy(conn, dataStream); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			runtime.HandleError(fmt.Errorf("error copying from remote stream to local connection: %v", err))
		}

		// inform the select below that the remote copy is done
		close(remoteDone)
	}()

	go func() {
		// inform server we're not sending any more data after copy unblocks
		defer dataStream.Close()

		// Copy from the local port to the remote side.
		if _, err := io.Copy(dataStream, conn); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			runtime.HandleError(fmt.Errorf("error copying from local connection to remote stream: %v", err))
			// break out of the select below without waiting for the other copy to finish
			close(localError)
		}
	}()
	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}

	// always expect something on errorChan (it may be nil)
	err = <-errorChan
	if err != nil {
		runtime.HandleError(err)
		pf.streamConn.Close()
	}
}
