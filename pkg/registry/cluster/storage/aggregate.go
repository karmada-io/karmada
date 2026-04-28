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

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	"github.com/karmada-io/karmada/pkg/util/lifted"
	"github.com/karmada-io/karmada/pkg/util/proxy"
)

func karmadaResourceLocation(restConfig *restclient.Config) (*url.URL, http.RoundTripper, error) {
	location, err := url.Parse(restConfig.Host)
	if err != nil {
		return nil, nil, err
	}

	transport, err := restclient.TransportFor(restConfig)
	if err != nil {
		return nil, nil, err
	}

	return location, transport, nil
}

type requestContext struct {
	clusterName      string
	location         *url.URL
	transport        http.RoundTripper
	impersonateToken string
	responseBody     []byte
	responseHeader   http.Header
}

// connectAllClusters returns a handler to proxy all clusters.
// Aggregates resources returned by all cluster. If resource names conflict,
// a conflict error is returned during handler processing.
func (r *ProxyREST) connectAllClusters(
	ctx context.Context,
	proxyPath string,
	secretGetter func(context.Context, string, string) (*corev1.Secret, error),
	responder rest.Responder,
) (http.Handler, error) {
	klog.V(4).Infof("Request resources with the proxyPath(%s)", proxyPath)
	proxyRequestInfo := lifted.NewRequestInfo(&http.Request{
		URL: &url.URL{Path: proxyPath},
	})

	// 1. for no resource request, proxy the request to karmada apiserver.
	if !proxyRequestInfo.IsResourceRequest {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			location := *r.karmadaLocation
			location.Path = path.Join(location.Path, proxyPath)
			location.RawQuery = req.URL.RawQuery

			handler := proxy.NewThrottledUpgradeAwareProxyHandler(&location, r.karmadaTransPort, true, false, responder)
			handler.ServeHTTP(rw, req)
		}), nil
	}

	clusterList, err := r.clusterLister(ctx)
	if err != nil {
		return nil, err
	}

	// 2. for resource request.
	if len(proxyRequestInfo.Name) != 0 {
		return requestWithResourceNameHandlerFunc(ctx, proxyPath, secretGetter, responder, clusterList), nil
	}
	return requestWithoutResourceNameHandlerFunc(ctx, proxyPath, secretGetter, clusterList), nil
}

func requestWithResourceNameHandlerFunc(
	ctx context.Context,
	proxyPath string,
	secretGetter func(context.Context, string, string) (*corev1.Secret, error),
	responder rest.Responder,
	clusterList *clusterapis.ClusterList,
) http.Handler {
	// For specific resource, get first to determine which cluster belong to,
	// and then proxy to the target member cluster.
	// Note: for resource creation requests, the resource name is not specified,
	// so the request to create a resource is not considered.
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		proxyRequestInfo := lifted.NewRequestInfo(&http.Request{
			URL:    &url.URL{Path: proxyPath, RawQuery: req.URL.RawQuery},
			Method: req.Method,
		})

		requester, exist := request.UserFrom(req.Context())
		if !exist {
			responsewriters.InternalError(rw, req, errors.New("no user found for request"))
			return
		}
		locker := sync.Mutex{}
		wg := sync.WaitGroup{}
		requestContexts := make([]requestContext, 0)
		for i := range clusterList.Items {
			wg.Add(1)
			cluster := clusterList.Items[i]
			go func(cluster *clusterapis.Cluster) {
				defer wg.Done()
				tlsConfig, err := proxy.GetTLSConfigForCluster(ctx, cluster, secretGetter)
				if err != nil {
					klog.Error(err)
					return
				}
				location, transport, err := proxy.Location(cluster, tlsConfig)
				if err != nil {
					klog.Error(err)
					return
				}
				impersonateToken, err := clusterImpersonateToken(ctx, cluster, secretGetter)
				if err != nil {
					klog.Errorf("failed to get impersonateToken for cluster %s: %v", cluster.Name, err)
					return
				}
				statusCode, err := doClusterRequest(http.MethodGet, requestURLStr(location, proxyRequestInfo), transport,
					requester, impersonateToken)
				if err != nil {
					klog.Errorf("failed to do request for cluster %s: %v", cluster.Name, err)
					return
				}
				if statusCode == http.StatusOK {
					locker.Lock()
					requestContexts = append(requestContexts, requestContext{
						clusterName:      cluster.Name,
						location:         location,
						transport:        transport,
						impersonateToken: impersonateToken,
					})
					locker.Unlock()
				}
			}(&cluster)
		}
		wg.Wait()
		switch len(requestContexts) {
		case 0:
			http.Error(rw, "resource not found or don't have permission to get it", http.StatusNotFound)
		case 1:
			reqCtx := requestContexts[0]
			setRequestHeader(req, requester, reqCtx.impersonateToken)
			reqCtx.location.Path = path.Join(reqCtx.location.Path, proxyPath)
			reqCtx.location.RawQuery = req.URL.RawQuery

			handler := proxy.NewThrottledUpgradeAwareProxyHandler(reqCtx.location, reqCtx.transport,
				true, false, responder)
			handler.ServeHTTP(rw, req)
		default:
			clusterNames := make([]string, len(requestContexts))
			for i, reqCtx := range requestContexts {
				clusterNames[i] = reqCtx.clusterName
			}
			http.Error(rw, fmt.Sprintf("conflict resource, exist in more than one cluster: %s",
				strings.Join(clusterNames, ",")), http.StatusConflict)
		}
	})
}

// nolint:gocyclo
func requestWithoutResourceNameHandlerFunc(
	ctx context.Context,
	proxyPath string,
	secretGetter func(context.Context, string, string) (*corev1.Secret, error),
	clusterList *clusterapis.ClusterList,
) http.Handler {
	// For uncertain resource names in processing, we need to make further judgments
	// based on the requested verb to determine whether to merge the request results
	// or proxy the request to the target cluster.
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		proxyRequestInfo := lifted.NewRequestInfo(&http.Request{
			URL:    &url.URL{Path: proxyPath, RawQuery: req.URL.RawQuery},
			Method: req.Method,
		})

		requester, exist := request.UserFrom(req.Context())
		if !exist {
			responsewriters.InternalError(rw, req, errors.New("no user found for request"))
			return
		}
		if proxyRequestInfo.Verb != "list" {
			http.Error(rw, fmt.Sprintf("Request verb %s is not support", proxyRequestInfo.Verb), http.StatusMethodNotAllowed)
			return
		}
		locker := sync.Mutex{}
		wg := sync.WaitGroup{}
		targetClusters := make([]requestContext, 0)
		for i := range clusterList.Items {
			wg.Add(1)
			cluster := clusterList.Items[i]
			go func(cluster *clusterapis.Cluster) {
				defer wg.Done()
				tlsConfig, err := proxy.GetTLSConfigForCluster(ctx, cluster, secretGetter)
				if err != nil {
					klog.Error(err)
					return
				}
				location, transport, err := proxy.Location(cluster, tlsConfig)
				if err != nil {
					klog.Error(err)
					return
				}
				impersonateToken, err := clusterImpersonateToken(ctx, cluster, secretGetter)
				if err != nil {
					klog.Errorf("failed to get impersonateToken for cluster %s: %v", cluster.Name, err)
					return
				}

				location.Path = path.Join(location.Path, proxyPath)
				location.RawQuery = req.URL.RawQuery

				simpleRequest, err := http.NewRequest(req.Method, location.String(), nil)
				if err != nil {
					klog.Errorf("failed to create request for cluster %s: %v", cluster.Name, err)
					return
				}
				// simpleRequest.Header = req.Header.Clone()
				setRequestHeader(simpleRequest, requester, impersonateToken)

				httpClient := &http.Client{Transport: transport}
				resp, err := httpClient.Do(simpleRequest)
				if err != nil {
					klog.Errorf("failed to do request for cluster %s: %v", cluster.Name, err)
					return
				}
				if resp.StatusCode != http.StatusOK {
					klog.Warningf("get resource not ok with cluster %s: %d", cluster.Name, resp.StatusCode)
					return
				}
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					klog.Errorf("unable to read content with cluster %s response: %v", cluster.Name, err)
					return
				}
				locker.Lock()
				targetClusters = append(targetClusters, requestContext{
					clusterName:    cluster.Name,
					responseBody:   body,
					responseHeader: resp.Header,
				})
				locker.Unlock()
				_ = resp.Body.Close()
			}(&cluster)
		}
		wg.Wait()
		if len(targetClusters) == 0 {
			http.Error(rw, "not found", http.StatusNotFound)
			return
		}

		var resObjList *unstructured.UnstructuredList
		for _, reqCtx := range targetClusters {
			objList := &unstructured.UnstructuredList{}
			err := objList.UnmarshalJSON(reqCtx.responseBody)
			if err != nil {
				klog.Errorf("Failed to unmarshal object list, error is: %v", err)
				continue
			}

			if resObjList == nil {
				resObjList = objList
				continue
			}
			resObjList.Items = append(resObjList.Items, objList.Items...)
		}
		resByte, err := resObjList.MarshalJSON()
		if err != nil {
			klog.Errorf("Failed to marshal object list, error is: %v", err)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Content-Length", strconv.Itoa(len(resByte)))
		for k, vs := range targetClusters[0].responseHeader {
			if rw.Header().Get(k) != "" {
				continue
			}
			for _, v := range vs {
				rw.Header().Set(k, v)
			}
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write(resByte)
	})
}

func clusterImpersonateToken(
	ctx context.Context,
	cluster *clusterapis.Cluster,
	secretGetter func(context.Context, string, string) (*corev1.Secret, error),
) (string, error) {
	if cluster.Spec.ImpersonatorSecretRef == nil {
		return "", fmt.Errorf("the impersonatorSecretRef of cluster is nil")
	}
	secret, err := secretGetter(ctx, cluster.Spec.ImpersonatorSecretRef.Namespace, cluster.Spec.ImpersonatorSecretRef.Name)
	if err != nil {
		return "", err
	}
	impersonateToken, err := proxy.ImpersonateToken(cluster.Name, secret)
	if err != nil {
		return "", err
	}
	return impersonateToken, nil
}

func doClusterRequest(
	method string,
	url string,
	transport http.RoundTripper,
	userInfo user.Info,
	impersonateToken string,
) (statusCode int, err error) {
	simpleRequest, err := http.NewRequest(method, url, nil)
	if err != nil {
		return 0, err
	}
	setRequestHeader(simpleRequest, userInfo, impersonateToken)

	httpClient := &http.Client{Transport: transport}
	resp, err := httpClient.Do(simpleRequest)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// requestURLStr returns the request resource url string.
func requestURLStr(location *url.URL, requestInfo *request.RequestInfo) string {
	parts := []string{requestInfo.APIPrefix}
	if requestInfo.APIGroup != "" {
		parts = append(parts, requestInfo.APIGroup)
	}
	parts = append(parts, requestInfo.APIVersion)
	if requestInfo.Namespace != "" {
		parts = append(parts, "namespaces", requestInfo.Namespace)
	}
	if requestInfo.Resource != "" {
		parts = append(parts, requestInfo.Resource)
	}
	if requestInfo.Name != "" {
		parts = append(parts, requestInfo.Name)
	}
	if requestInfo.Subresource != "" &&
		requestInfo.Subresource != "exec" && requestInfo.Subresource != "log" {
		parts = append(parts, requestInfo.Subresource)
	}
	return location.ResolveReference(&url.URL{Path: strings.Join(parts, "/")}).String()
}

func setRequestHeader(req *http.Request, userInfo user.Info, impersonateToken string) {
	req.Header.Set(authenticationv1.ImpersonateUserHeader, userInfo.GetName())
	for _, group := range userInfo.GetGroups() {
		if !proxy.SkipGroup(group) {
			req.Header.Add(authenticationv1.ImpersonateGroupHeader, group)
		}
	}
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", impersonateToken))
}
