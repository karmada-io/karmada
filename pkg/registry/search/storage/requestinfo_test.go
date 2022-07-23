package storage

import (
	"reflect"
	"strings"
	"testing"

	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
)

func Test_parseK8sNativeResourceInfo(t *testing.T) {
	type args struct {
		reqParts []string
	}
	tests := []struct {
		name    string
		args    args
		want    *genericrequest.RequestInfo
		wantErr bool
	}{
		{
			name: "len(reqParts) < 3",
			args: args{
				reqParts: []string{"v1", "deployments"},
			},
			want:    &genericrequest.RequestInfo{IsResourceRequest: false, Path: strings.Join([]string{"v1", "deployments"}, "/")},
			wantErr: false,
		},
		{
			name: "apiPrefix not in [api, apis]",
			args: args{
				reqParts: []string{"apps", "v1", "deployments"},
			},
			want:    &genericrequest.RequestInfo{IsResourceRequest: false, Path: strings.Join([]string{"apps", "v1", "deployments"}, "/")},
			wantErr: false,
		},
		{
			name: "apiPrefix is not api",
			args: args{
				reqParts: []string{"apis", "apps", "v1", "deployments"},
			},
			want: &genericrequest.RequestInfo{
				IsResourceRequest: true,
				Path:              strings.Join([]string{"apis", "apps", "v1", "deployments"}, "/"),
				APIPrefix:         "apis",
				APIGroup:          "apps",
				APIVersion:        "v1",
				Resource:          "deployments",
			},
			wantErr: false,
		},
		{
			name: "request namespace scope  resource list",
			args: args{
				reqParts: []string{"apis", "apps", "v1", "namespaces", "default", "deployments"},
			},
			want: &genericrequest.RequestInfo{
				IsResourceRequest: true,
				Path:              strings.Join([]string{"apis", "apps", "v1", "namespaces", "default", "deployments"}, "/"),
				APIPrefix:         "apis",
				APIGroup:          "apps",
				APIVersion:        "v1",
				Namespace:         "default",
				Resource:          "deployments",
			},
		},
		{
			name: "request a namespace scope  resource",
			args: args{
				reqParts: []string{"apis", "apps", "v1", "namespaces", "default", "deployments", "foo"},
			},
			want: &genericrequest.RequestInfo{
				IsResourceRequest: true,
				Path:              strings.Join([]string{"apis", "apps", "v1", "namespaces", "default", "deployments", "foo"}, "/"),
				APIPrefix:         "apis",
				APIGroup:          "apps",
				APIVersion:        "v1",
				Namespace:         "default",
				Resource:          "deployments",
				Name:              "foo",
			},
		},
		{
			name: "resource is namespaces",
			args: args{
				reqParts: []string{"api", "v1", "namespaces"},
			},
			want: &genericrequest.RequestInfo{
				IsResourceRequest: true,
				Path:              strings.Join([]string{"api", "v1", "namespaces"}, "/"),
				APIPrefix:         "api",
				APIVersion:        "v1",
				Resource:          "namespaces",
			},
		},
		{
			name: "resource is a specified namespaces",
			args: args{
				reqParts: []string{"api", "v1", "namespaces", "default"},
			},
			want: &genericrequest.RequestInfo{
				IsResourceRequest: true,
				Path:              strings.Join([]string{"api", "v1", "namespaces", "default"}, "/"),
				APIPrefix:         "api",
				APIVersion:        "v1",
				Resource:          "namespaces",
				Name:              "default",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseK8sNativeResourceInfo(tt.args.reqParts)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseK8sNativeResourceInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseK8sNativeResourceInfo() got = %v, want %v", got, tt.want)
			}
		})
	}
}
