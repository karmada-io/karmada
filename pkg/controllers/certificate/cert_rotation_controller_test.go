/*
Copyright 2024 The Karmada Authors.

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

package certificate

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

// cert will expire on 2124-07-28T02:18:16Z
var testCA = `-----BEGIN CERTIFICATE-----
MIIDbTCCAlWgAwIBAgIUQkIQIcbPnAqI8ucdX54k1QhlzQ4wDQYJKoZIhvcNAQEL
BQAwNDEZMBcGA1UEAwwQa3ViZXJuZXRlcy1hZG1pbjEXMBUGA1UECgwOc3lzdGVt
Om1hc3RlcnMwIBcNMjQwODIxMDIxODE2WhgPMjEyNDA3MjgwMjE4MTZaMDQxGTAX
BgNVBAMMEGt1YmVybmV0ZXMtYWRtaW4xFzAVBgNVBAoMDnN5c3RlbTptYXN0ZXJz
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAqVWggRDZWNvCgNEMoncb
ASN3RNpZfA+cK4vNC/D2R9m5ATR/ZGt06aP3zWLkqc43MmwsLcFDy/wY+hB50/It
Zqjt5EaIl1jqZnRXuEXe5phq/fZICSM2vL9tt0JX9L9c5LedSWJwSZ8gvjpwQacK
SGMy+HM5lC5Ta3bOR98sTEyFG6Z8kX9KT2HgYsveShO242TRUSJPKW+xocJjxqL+
GFHKoZp4D+yYkZ2dahHvPiSCxe9WDXKbpZRPTxNb/EMkJ6YOuU8N2QW44u9Lx1y7
jIAPL6vGcUJeo2UhvyShGKzPrI1tGSWnjKxKCOv8rK5NPuhIXTXDHhTCDB4/r6xt
xQIDAQABo3UwczAdBgNVHQ4EFgQU3vrp4HTLqAPRBqVbo9CV53KjlQkwHwYDVR0j
BBgwFoAU3vrp4HTLqAPRBqVbo9CV53KjlQkwDgYDVR0PAQH/BAQDAgWgMBMGA1Ud
JQQMMAoGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEB
ADy6VgqstiGXTxV0VzyXtHqVkHX2GNl58HYwC8ti5uy/T7U/WhzOmjDMxoZonA+m
wzE25Dp1J7DN2y02skHoYMP8u0fsDBACWtXxFhwUja+De1CiEZCGhDUeMNS3ka2j
4z9Ow3yChanJXmR6n91hA5TGJ4uk9eFrQgKoLqZ/poRaoxj6861XKWiJS1Wvrz1g
fmbSjVIn4QFA9f611iwS/wGNHJ1dLUza9WuiQeOjculCqxqBl4+kQWmRBcmOkse2
+KuZJMIMJfS2521AZO35EgXblA2BG1TgZZz6i3E0NMjzi+T1NIMwXawiWXDt4W/l
umubw9aqN/m5NUa3hZ6XmXQ=
-----END CERTIFICATE-----`

func makeFakeCertRotationController(threshold float64) *CertRotationController {
	client := clientfake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
		&clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Finalizers: []string{util.ClusterControllerFinalizer}},
			Spec: clusterv1alpha1.ClusterSpec{
				APIEndpoint: "https://127.0.0.1",
				SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
			},
			Status: clusterv1alpha1.ClusterStatus{
				Conditions: []metav1.Condition{
					{
						Type:   clusterv1alpha1.ClusterConditionReady,
						Status: metav1.ConditionTrue,
					},
				},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
			Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: []byte(testCA)},
		}).Build()
	return &CertRotationController{
		Client:                             client,
		KubeClient:                         fake.NewSimpleClientset(),
		CertRotationRemainingTimeThreshold: threshold,
		ClusterClientSetFunc:               util.NewClusterClientSet,
		KarmadaKubeconfigNamespace:         "karmada-system",
	}
}

func TestCertRotationController_Reconcile(t *testing.T) {
	tests := []struct {
		name    string
		cluster *clusterv1alpha1.Cluster
		del     bool
		want    controllerruntime.Result
		wantErr bool
	}{
		{
			name: "cluster not found",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test1",
				},
			},
			want:    controllerruntime.Result{},
			wantErr: false,
		},
		{
			name: "cluster is deleted",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
			},
			del:     true,
			want:    controllerruntime.Result{},
			wantErr: false,
		},
		{
			name: "get secret failed",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := makeFakeCertRotationController(0)
			if tt.del {
				if err := c.Client.Delete(context.Background(), tt.cluster); err != nil {
					t.Fatalf("delete cluster failed, error %v", err)
				}
			}

			req := controllerruntime.Request{NamespacedName: client.ObjectKey{Namespace: "", Name: tt.cluster.Name}}
			got, err := c.Reconcile(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CertRotationController.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CertRotationController.Reconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCertRotationController_syncCertRotation(t *testing.T) {
	tests := []struct {
		name      string
		secret    *corev1.Secret
		signer    bool
		threshold float64
		wantErr   bool
	}{
		{
			name: "should not rotate cert",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret"},
				Data: map[string][]byte{"karmada-kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMvakNDQWVhZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRJME1EZ3hOVEEyTVRZd01Gb1hEVE0wTURneE16QTJNVFl3TUZvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTndZCjU3blNJNDgwMUZIYmtYVUhpUldTTmV3UUxRTTZQbTB5YXArd1JXR2J3emU3US9rbjl0L2xBUWwxdW1aa2ZRalUKVHgyZHV6cFpXQkRndnAreTVBNndaUyt2VTVhSFY4dE1QRi9ocHRVczB1VW11YmQ2OEs4ZnNuREd6bnJwKzdpQwo3R2VyVzB2NDNTdnpqT0dibDQ2Nlp5cXFPRmt5VVhPQ1pVWFJMbWkyMVNrbS9iU2RFS3FDZXBtRDFNSEUwVyttCkJOOXBQeFJOU1dCZGNkSFVqR29odUUrUVBJQXlDWEtBdlNlWDBOZDd6Q1Ayd1dFRE5aSmxRS0REUnFUUHdDS3QKMW9TaDdEeWhvQ0l6clBtNENIcVNHSEJCNnVORmNEZjdpNGhVY09SdW5JMHlVUEsya2FDUmdqTkZKYkJLL29SNApoSFl0SFJwUkN3b244Q3A4dWRFQ0F3RUFBYU5aTUZjd0RnWURWUjBQQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCCi93UUZNQU1CQWY4d0hRWURWUjBPQkJZRUZESUZTYXhZNDc1WlZaTlp3dGdwOU1yeFBrU2ZNQlVHQTFVZEVRUU8KTUF5Q0NtdDFZbVZ5Ym1WMFpYTXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSnY3Ymw1L2plVlFZZkxWblByKwpBelVYNmxXdXNHajQ0b09ma2xtZGZTTmxLU3ZGUjVRMi9rWkVQZXZnU3NzZVdIWnVucmZTYkIwVDdWYjdkUkNQCjVRMTk4aUNnZDFwNm0wdXdOVGpSRi85MHhzYUdVOHBQTFQxeHlrMDlCVDc0WVVENnNtOHFVSWVrWFU0U3hlU2oKWjk3VU13azVoZndXUWpqTFc1UklwNW1qZjR2aU1uWXB6SDB4bDREV3Jka1AxbTFCdkZvWmhFMEVaKzlWcGNPYwprNTN4ZkxUR3A2S1UrQ0w4RU5teXFCeTJNcVBXdjRQKzVTZ0hldlY3Ym1WdktuMkx0blExTHdCcDdsdldYb1JRCmUzQm83d3hnSUU0Rnl0VUU4enRaS2ZJSDZPY3VzNWJGY283cGw5ckhnK1lBMHM0Y0JldjZ2UlQwODkyYUpHYmUKZnFRPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    server: https://192.168.0.180:56016
  name: kind-member1
contexts:
- context:
    cluster: kind-member1
    user: kind-member1
  name: member1
current-context: member1
kind: Config
preferences: {}
users:
- name: kind-member1
  user:
    ## cert will expire on 2124-07-28T02:18:16Z
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURiVENDQWxXZ0F3SUJBZ0lVUWtJUUljYlBuQXFJOHVjZFg1NGsxUWhselE0d0RRWUpLb1pJaHZjTkFRRUwKQlFBd05ERVpNQmNHQTFVRUF3d1FhM1ZpWlhKdVpYUmxjeTFoWkcxcGJqRVhNQlVHQTFVRUNnd09jM2x6ZEdWdApPbTFoYzNSbGNuTXdJQmNOTWpRd09ESXhNREl4T0RFMldoZ1BNakV5TkRBM01qZ3dNakU0TVRaYU1EUXhHVEFYCkJnTlZCQU1NRUd0MVltVnlibVYwWlhNdFlXUnRhVzR4RnpBVkJnTlZCQW9NRG5ONWMzUmxiVHB0WVhOMFpYSnoKTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUFxVldnZ1JEWldOdkNnTkVNb25jYgpBU04zUk5wWmZBK2NLNHZOQy9EMlI5bTVBVFIvWkd0MDZhUDN6V0xrcWM0M01td3NMY0ZEeS93WStoQjUwL0l0ClpxanQ1RWFJbDFqcVpuUlh1RVhlNXBocS9mWklDU00ydkw5dHQwSlg5TDljNUxlZFNXSndTWjhndmpwd1FhY0sKU0dNeStITTVsQzVUYTNiT1I5OHNURXlGRzZaOGtYOUtUMkhnWXN2ZVNoTzI0MlRSVVNKUEtXK3hvY0pqeHFMKwpHRkhLb1pwNEQreVlrWjJkYWhIdlBpU0N4ZTlXRFhLYnBaUlBUeE5iL0VNa0o2WU91VThOMlFXNDR1OUx4MXk3CmpJQVBMNnZHY1VKZW8yVWh2eVNoR0t6UHJJMXRHU1duakt4S0NPdjhySzVOUHVoSVhUWERIaFRDREI0L3I2eHQKeFFJREFRQUJvM1V3Y3pBZEJnTlZIUTRFRmdRVTN2cnA0SFRMcUFQUkJxVmJvOUNWNTNLamxRa3dId1lEVlIwagpCQmd3Rm9BVTN2cnA0SFRMcUFQUkJxVmJvOUNWNTNLamxRa3dEZ1lEVlIwUEFRSC9CQVFEQWdXZ01CTUdBMVVkCkpRUU1NQW9HQ0NzR0FRVUZCd01DTUF3R0ExVWRFd0VCL3dRQ01BQXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUIKQUR5NlZncXN0aUdYVHhWMFZ6eVh0SHFWa0hYMkdObDU4SFl3Qzh0aTV1eS9UN1UvV2h6T21qRE14b1pvbkErbQp3ekUyNURwMUo3RE4yeTAyc2tIb1lNUDh1MGZzREJBQ1d0WHhGaHdVamErRGUxQ2lFWkNHaERVZU1OUzNrYTJqCjR6OU93M3lDaGFuSlhtUjZuOTFoQTVUR0o0dWs5ZUZyUWdLb0xxWi9wb1Jhb3hqNjg2MVhLV2lKUzFXdnJ6MWcKZm1iU2pWSW40UUZBOWY2MTFpd1Mvd0dOSEoxZExVemE5V3VpUWVPamN1bENxeHFCbDQra1FXbVJCY21Pa3NlMgorS3VaSk1JTUpmUzI1MjFBWk8zNUVnWGJsQTJCRzFUZ1paejZpM0UwTk1qemkrVDFOSU13WGF3aVdYRHQ0Vy9sCnVtdWJ3OWFxTi9tNU5VYTNoWjZYbVhRPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0t
    client-key-data: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ3BWYUNCRU5sWTI4S0EKMFF5aWR4c0JJM2RFMmxsOEQ1d3JpODBMOFBaSDJia0JOSDlrYTNUcG8vZk5ZdVNwempjeWJDd3R3VVBML0JqNgpFSG5UOGkxbXFPM2tSb2lYV09wbWRGZTRSZDdtbUdyOTlrZ0pJemE4djIyM1FsZjB2MXprdDUxSlluQkpueUMrCk9uQkJwd3BJWXpMNGN6bVVMbE5yZHM1SDN5eE1USVVicG55UmYwcFBZZUJpeTk1S0U3YmpaTkZSSWs4cGI3R2gKd21QR292NFlVY3FobW5nUDdKaVJuWjFxRWU4K0pJTEY3MVlOY3B1bGxFOVBFMXY4UXlRbnBnNjVUdzNaQmJqaQo3MHZIWEx1TWdBOHZxOFp4UWw2alpTRy9KS0VZck0rc2pXMFpKYWVNckVvSTYveXNyazArNkVoZE5jTWVGTUlNCkhqK3ZyRzNGQWdNQkFBRUNnZ0VBSTdLaGU1UUp2ZW5XUDBIUzRBMHI3RG1GMDBZVXgwcWpLYXIzTnlVOVJqaG8KQUJFSktpcGRJMFFsNFc2UHRoeDdGbTRuZ2gzVUpSU29UMDlaMzR5V2RhWDNRTUI5MnlvcmdCM1d3RW82aTNKbQpXOU9uckFWNGJLSU9oeXU5VHlOb2VlOGJnWFQzSnc0YzRQMkEzTlpTSEtDTkJrT0VSL0RjTlROK21UZzdKbnBDCnMvVmoyd2pibllQNmt6MVRTcEVjRksrb3NnYldXQ1AxVDFUeFRFN1k5VlBjbWhibzU5Lzdxc2EzaE8vUjgxRysKQ0VxU3U1emgrQmJvRFZHUStpZFV3OGtqUlhUS2MzWFBWb0R6SmR6cUtJVUYwTkc1Nm4wdGNoZkVEMUpWS09PSQp5a3REdjM1Qi9JWEkwYzk1UHpJN0crOWJvSHM1aW9BcUZKUlo2bllJQVFLQmdRRHNvTmJRc0p1U1pVcW9ZZTBLCkhERFpXS3F5NUM3Q1IvOFNXTXcxeFQzdnJVV2t2Wm5TZUJHUnpvYUEvZmRXQ3lGWHdBd1ZkS0xFSzFkdW5UWDEKUkQ4Zk9odFBDdTdiaXVvV2l4YlpMcGRPUXVzZlhRcDNHUWtoOUNIQVRPc1pML0tkMmxSd0F6dHpGNTZkVHRtdQplZFIxVENiTEVZK1A1Vzg0MXhjV2Y4OEh3UUtCZ1FDM01uaXBXY0xWNDBlS2xZMEg2alhBMDByWCs3V1U1M21RClFKNXAzbWlxSW5qRWlmV2ZQZHh4Y0hyVWRndzlpRk9pVUVvNENIQnNnam5wRU5wUjVhcUJpTzRWUFBuRXdXM2EKSmJ5eWdmRW4wREdBci9PdFpTTUF6OGN4NFVnZmZpSmZPSk8rZXRNemhDMXlMcFAwR05oM2UyRzlDZ0M3eVhDSQpGT1BvdWlzSEJRS0JnSGROT0VFTGFjUkxrWEtIdk0wR0haTFhZMmpDSnRrSkY0OFdlZzc2SFJvRUVFTFkzUDhDClRrbG5DT1ZzSmhHWmx2djQ5WjZ6cVlTaUhYakZobmpjS2I4Q3V0WUZPeHd4VTRoK0k4em44cDBnbkE2NkNCYTMKNXFUWncxS0M5VjFEa1YwSXdOMmdvNDZKY0F6N3ZrQjdhQ1NqZWtPVDNQKzl1Mis2OGdjRDlVdUJBb0dBUjkxdgp3aGRwUEJpZG52ck55VllTWWlOQkQvczVIMEd5eVdqZisrMzRwdzFBelBERnZ3TTRiL1BNNjMybmpaZm1IeDFhCkVDTVhYeW15NS8vcGRRa2dXeEpKTzJHaEpaTXZzY3p0K2lUSlluSGtpWFA4cG4rdlBJbEZ2Z1ovRVlPY25qZ0cKbFVsL2dvME9ldVZVdXdQb0h1N3l4NEtlQ1F5YnJYWnNkWVphakxVQ2dZRUEyeEhLenJ3T1FwMlNNbXlzOFR1bgpZbm54bzMwV0diQzNpYVJIZkQ0TGRvNC9iNHY2TWtkMG9wdXBoMzArQjduRkZoRjlWSVplbVdGWkMrcXNKWHdLCmNwOGMzdjBNZFJkeUtTcTJ4Ly9ZL2s1KytaeU5DSXRZdzdhRy91Wks4TlZyY3h0c2hXOXBobDFJTThXTjBUYmgKNmxwd2xWZGFha0RPY09ZNjE2clRGOXM9Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0=`)},
			},
			threshold: 0,
			signer:    false,
			wantErr:   false,
		},
		{
			name: "update secret failed",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret"},
				Data: map[string][]byte{"karmada-kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMvakNDQWVhZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRJME1EZ3hOVEEyTVRZd01Gb1hEVE0wTURneE16QTJNVFl3TUZvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTndZCjU3blNJNDgwMUZIYmtYVUhpUldTTmV3UUxRTTZQbTB5YXArd1JXR2J3emU3US9rbjl0L2xBUWwxdW1aa2ZRalUKVHgyZHV6cFpXQkRndnAreTVBNndaUyt2VTVhSFY4dE1QRi9ocHRVczB1VW11YmQ2OEs4ZnNuREd6bnJwKzdpQwo3R2VyVzB2NDNTdnpqT0dibDQ2Nlp5cXFPRmt5VVhPQ1pVWFJMbWkyMVNrbS9iU2RFS3FDZXBtRDFNSEUwVyttCkJOOXBQeFJOU1dCZGNkSFVqR29odUUrUVBJQXlDWEtBdlNlWDBOZDd6Q1Ayd1dFRE5aSmxRS0REUnFUUHdDS3QKMW9TaDdEeWhvQ0l6clBtNENIcVNHSEJCNnVORmNEZjdpNGhVY09SdW5JMHlVUEsya2FDUmdqTkZKYkJLL29SNApoSFl0SFJwUkN3b244Q3A4dWRFQ0F3RUFBYU5aTUZjd0RnWURWUjBQQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCCi93UUZNQU1CQWY4d0hRWURWUjBPQkJZRUZESUZTYXhZNDc1WlZaTlp3dGdwOU1yeFBrU2ZNQlVHQTFVZEVRUU8KTUF5Q0NtdDFZbVZ5Ym1WMFpYTXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSnY3Ymw1L2plVlFZZkxWblByKwpBelVYNmxXdXNHajQ0b09ma2xtZGZTTmxLU3ZGUjVRMi9rWkVQZXZnU3NzZVdIWnVucmZTYkIwVDdWYjdkUkNQCjVRMTk4aUNnZDFwNm0wdXdOVGpSRi85MHhzYUdVOHBQTFQxeHlrMDlCVDc0WVVENnNtOHFVSWVrWFU0U3hlU2oKWjk3VU13azVoZndXUWpqTFc1UklwNW1qZjR2aU1uWXB6SDB4bDREV3Jka1AxbTFCdkZvWmhFMEVaKzlWcGNPYwprNTN4ZkxUR3A2S1UrQ0w4RU5teXFCeTJNcVBXdjRQKzVTZ0hldlY3Ym1WdktuMkx0blExTHdCcDdsdldYb1JRCmUzQm83d3hnSUU0Rnl0VUU4enRaS2ZJSDZPY3VzNWJGY283cGw5ckhnK1lBMHM0Y0JldjZ2UlQwODkyYUpHYmUKZnFRPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    server: https://192.168.0.180:56016
  name: kind-member1
contexts:
- context:
    cluster: kind-member1
    user: kind-member1
  name: member1
current-context: member1
kind: Config
preferences: {}
users:
- name: kind-member1
  user:
    ## cert will expire on 2124-07-28T02:18:16Z
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURiVENDQWxXZ0F3SUJBZ0lVUWtJUUljYlBuQXFJOHVjZFg1NGsxUWhselE0d0RRWUpLb1pJaHZjTkFRRUwKQlFBd05ERVpNQmNHQTFVRUF3d1FhM1ZpWlhKdVpYUmxjeTFoWkcxcGJqRVhNQlVHQTFVRUNnd09jM2x6ZEdWdApPbTFoYzNSbGNuTXdJQmNOTWpRd09ESXhNREl4T0RFMldoZ1BNakV5TkRBM01qZ3dNakU0TVRaYU1EUXhHVEFYCkJnTlZCQU1NRUd0MVltVnlibVYwWlhNdFlXUnRhVzR4RnpBVkJnTlZCQW9NRG5ONWMzUmxiVHB0WVhOMFpYSnoKTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUFxVldnZ1JEWldOdkNnTkVNb25jYgpBU04zUk5wWmZBK2NLNHZOQy9EMlI5bTVBVFIvWkd0MDZhUDN6V0xrcWM0M01td3NMY0ZEeS93WStoQjUwL0l0ClpxanQ1RWFJbDFqcVpuUlh1RVhlNXBocS9mWklDU00ydkw5dHQwSlg5TDljNUxlZFNXSndTWjhndmpwd1FhY0sKU0dNeStITTVsQzVUYTNiT1I5OHNURXlGRzZaOGtYOUtUMkhnWXN2ZVNoTzI0MlRSVVNKUEtXK3hvY0pqeHFMKwpHRkhLb1pwNEQreVlrWjJkYWhIdlBpU0N4ZTlXRFhLYnBaUlBUeE5iL0VNa0o2WU91VThOMlFXNDR1OUx4MXk3CmpJQVBMNnZHY1VKZW8yVWh2eVNoR0t6UHJJMXRHU1duakt4S0NPdjhySzVOUHVoSVhUWERIaFRDREI0L3I2eHQKeFFJREFRQUJvM1V3Y3pBZEJnTlZIUTRFRmdRVTN2cnA0SFRMcUFQUkJxVmJvOUNWNTNLamxRa3dId1lEVlIwagpCQmd3Rm9BVTN2cnA0SFRMcUFQUkJxVmJvOUNWNTNLamxRa3dEZ1lEVlIwUEFRSC9CQVFEQWdXZ01CTUdBMVVkCkpRUU1NQW9HQ0NzR0FRVUZCd01DTUF3R0ExVWRFd0VCL3dRQ01BQXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUIKQUR5NlZncXN0aUdYVHhWMFZ6eVh0SHFWa0hYMkdObDU4SFl3Qzh0aTV1eS9UN1UvV2h6T21qRE14b1pvbkErbQp3ekUyNURwMUo3RE4yeTAyc2tIb1lNUDh1MGZzREJBQ1d0WHhGaHdVamErRGUxQ2lFWkNHaERVZU1OUzNrYTJqCjR6OU93M3lDaGFuSlhtUjZuOTFoQTVUR0o0dWs5ZUZyUWdLb0xxWi9wb1Jhb3hqNjg2MVhLV2lKUzFXdnJ6MWcKZm1iU2pWSW40UUZBOWY2MTFpd1Mvd0dOSEoxZExVemE5V3VpUWVPamN1bENxeHFCbDQra1FXbVJCY21Pa3NlMgorS3VaSk1JTUpmUzI1MjFBWk8zNUVnWGJsQTJCRzFUZ1paejZpM0UwTk1qemkrVDFOSU13WGF3aVdYRHQ0Vy9sCnVtdWJ3OWFxTi9tNU5VYTNoWjZYbVhRPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0t
    client-key-data: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ3BWYUNCRU5sWTI4S0EKMFF5aWR4c0JJM2RFMmxsOEQ1d3JpODBMOFBaSDJia0JOSDlrYTNUcG8vZk5ZdVNwempjeWJDd3R3VVBML0JqNgpFSG5UOGkxbXFPM2tSb2lYV09wbWRGZTRSZDdtbUdyOTlrZ0pJemE4djIyM1FsZjB2MXprdDUxSlluQkpueUMrCk9uQkJwd3BJWXpMNGN6bVVMbE5yZHM1SDN5eE1USVVicG55UmYwcFBZZUJpeTk1S0U3YmpaTkZSSWs4cGI3R2gKd21QR292NFlVY3FobW5nUDdKaVJuWjFxRWU4K0pJTEY3MVlOY3B1bGxFOVBFMXY4UXlRbnBnNjVUdzNaQmJqaQo3MHZIWEx1TWdBOHZxOFp4UWw2alpTRy9KS0VZck0rc2pXMFpKYWVNckVvSTYveXNyazArNkVoZE5jTWVGTUlNCkhqK3ZyRzNGQWdNQkFBRUNnZ0VBSTdLaGU1UUp2ZW5XUDBIUzRBMHI3RG1GMDBZVXgwcWpLYXIzTnlVOVJqaG8KQUJFSktpcGRJMFFsNFc2UHRoeDdGbTRuZ2gzVUpSU29UMDlaMzR5V2RhWDNRTUI5MnlvcmdCM1d3RW82aTNKbQpXOU9uckFWNGJLSU9oeXU5VHlOb2VlOGJnWFQzSnc0YzRQMkEzTlpTSEtDTkJrT0VSL0RjTlROK21UZzdKbnBDCnMvVmoyd2pibllQNmt6MVRTcEVjRksrb3NnYldXQ1AxVDFUeFRFN1k5VlBjbWhibzU5Lzdxc2EzaE8vUjgxRysKQ0VxU3U1emgrQmJvRFZHUStpZFV3OGtqUlhUS2MzWFBWb0R6SmR6cUtJVUYwTkc1Nm4wdGNoZkVEMUpWS09PSQp5a3REdjM1Qi9JWEkwYzk1UHpJN0crOWJvSHM1aW9BcUZKUlo2bllJQVFLQmdRRHNvTmJRc0p1U1pVcW9ZZTBLCkhERFpXS3F5NUM3Q1IvOFNXTXcxeFQzdnJVV2t2Wm5TZUJHUnpvYUEvZmRXQ3lGWHdBd1ZkS0xFSzFkdW5UWDEKUkQ4Zk9odFBDdTdiaXVvV2l4YlpMcGRPUXVzZlhRcDNHUWtoOUNIQVRPc1pML0tkMmxSd0F6dHpGNTZkVHRtdQplZFIxVENiTEVZK1A1Vzg0MXhjV2Y4OEh3UUtCZ1FDM01uaXBXY0xWNDBlS2xZMEg2alhBMDByWCs3V1U1M21RClFKNXAzbWlxSW5qRWlmV2ZQZHh4Y0hyVWRndzlpRk9pVUVvNENIQnNnam5wRU5wUjVhcUJpTzRWUFBuRXdXM2EKSmJ5eWdmRW4wREdBci9PdFpTTUF6OGN4NFVnZmZpSmZPSk8rZXRNemhDMXlMcFAwR05oM2UyRzlDZ0M3eVhDSQpGT1BvdWlzSEJRS0JnSGROT0VFTGFjUkxrWEtIdk0wR0haTFhZMmpDSnRrSkY0OFdlZzc2SFJvRUVFTFkzUDhDClRrbG5DT1ZzSmhHWmx2djQ5WjZ6cVlTaUhYakZobmpjS2I4Q3V0WUZPeHd4VTRoK0k4em44cDBnbkE2NkNCYTMKNXFUWncxS0M5VjFEa1YwSXdOMmdvNDZKY0F6N3ZrQjdhQ1NqZWtPVDNQKzl1Mis2OGdjRDlVdUJBb0dBUjkxdgp3aGRwUEJpZG52ck55VllTWWlOQkQvczVIMEd5eVdqZisrMzRwdzFBelBERnZ3TTRiL1BNNjMybmpaZm1IeDFhCkVDTVhYeW15NS8vcGRRa2dXeEpKTzJHaEpaTXZzY3p0K2lUSlluSGtpWFA4cG4rdlBJbEZ2Z1ovRVlPY25qZ0cKbFVsL2dvME9ldVZVdXdQb0h1N3l4NEtlQ1F5YnJYWnNkWVphakxVQ2dZRUEyeEhLenJ3T1FwMlNNbXlzOFR1bgpZbm54bzMwV0diQzNpYVJIZkQ0TGRvNC9iNHY2TWtkMG9wdXBoMzArQjduRkZoRjlWSVplbVdGWkMrcXNKWHdLCmNwOGMzdjBNZFJkeUtTcTJ4Ly9ZL2s1KytaeU5DSXRZdzdhRy91Wks4TlZyY3h0c2hXOXBobDFJTThXTjBUYmgKNmxwd2xWZGFha0RPY09ZNjE2clRGOXM9Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0=`)},
			},
			threshold: 1,
			signer:    true,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := makeFakeCertRotationController(tt.threshold)
			cc, err := util.NewClusterClientSet("test", c.Client, &util.ClientOption{})
			if err != nil {
				t.Fatal(err)
			}
			c.ClusterClient = cc

			if tt.signer {
				go mockCSRSigner(c.KubeClient)
			}

			if err := c.syncCertRotation(context.Background(), tt.secret); (err != nil) != tt.wantErr {
				t.Errorf("CertRotationController.syncCertRotation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func mockCSRSigner(cs kubernetes.Interface) {
	for {
		select {
		case <-time.After(10 * time.Second):
			return
		default:
			csrL, _ := cs.CertificatesV1().CertificateSigningRequests().List(context.Background(), metav1.ListOptions{})
			for _, csr := range csrL.Items {
				t := csr.DeepCopy()
				t.Status.Certificate = []byte(testCA)
				if _, err := cs.CertificatesV1().CertificateSigningRequests().UpdateStatus(context.Background(), t, metav1.UpdateOptions{}); err != nil {
					return
				}
			}
		}
	}
}

func Test_getCertValidityPeriod(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		certTime   []map[string]time.Time
		wantBefore time.Time
		wantAfter  time.Time
		wantErr    bool
	}{
		{
			name: "get cert validity period success",
			certTime: []map[string]time.Time{
				{
					"notBefore": now.Add(-36 * time.Hour).UTC().Truncate(time.Second),
					"notAfter":  now.Add(72 * time.Hour).UTC().Truncate(time.Second),
				},
				{
					"notBefore": now.Add(-24 * time.Hour).UTC().Truncate(time.Second),
					"notAfter":  now.Add(36 * time.Hour).UTC().Truncate(time.Second),
				},
			},
			wantBefore: now.Add(-24 * time.Hour).UTC().Truncate(time.Second),
			wantAfter:  now.Add(36 * time.Hour).UTC().Truncate(time.Second),
			wantErr:    false,
		},
		{
			name:     "parse cert fail",
			certTime: []map[string]time.Time{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certData := []byte{}
			for _, ct := range tt.certTime {
				cert, err := newMockCert(ct["notBefore"], ct["notAfter"])
				if err != nil {
					t.Fatal(err)
				}
				certData = append(certData, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})...)
			}

			notBefore, notAfter, err := getCertValidityPeriod(certData)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !tt.wantBefore.Equal(*notBefore) || !tt.wantAfter.Equal(*notAfter) {
				t.Errorf("got notBefore=%s, notAfter=%s; want notBefore=%s, notAfter=%s", notBefore, notAfter, tt.wantBefore, tt.wantAfter)
			}
		})
	}
}

func newMockCert(notBefore, notAfter time.Time) (*x509.Certificate, error) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	testSigner, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "karmada.com",
			Organization: []string{"karmada"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certData, err := x509.CreateCertificate(rand.Reader, &template, &template, &testSigner.PublicKey, testSigner)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(certData)
}
