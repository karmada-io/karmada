/*
Copyright 2026 The Karmada Authors.

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

package base

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kindexec "sigs.k8s.io/kind/pkg/exec"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const (
	// Time for the apiserver's SA token cache to flush after revocation.
	revocationCacheFlushWait = 15 * time.Second
	// Duration to assert the cluster remains Ready (short-lived path unaffected).
	clusterStableWindow = 15 * time.Second
)

// Verifies push-mode informers recover after a member cluster token rotation.
var _ = framework.SerialDescribe("push-mode token rotation", func() {
	var (
		targetCluster  string
		deploymentNS   string
		deploymentName string
		deployment     *appsv1.Deployment
		policy         *policyv1alpha1.PropagationPolicy
	)

	ginkgo.BeforeEach(func() {
		pushClusters := framework.ClusterNamesWithSyncMode(clusterv1alpha1.Push)
		gomega.Expect(pushClusters).ShouldNot(gomega.BeEmpty(), "need at least one push-mode cluster")
		targetCluster = pushClusters[0]

		deploymentNS = testNamespace
		deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
		deployment = testhelper.NewDeployment(deploymentNS, deploymentName)

		policy = testhelper.NewPropagationPolicy(deploymentNS, deploymentName, []policyv1alpha1.ResourceSelector{
			{APIVersion: deployment.APIVersion, Kind: deployment.Kind, Name: deployment.Name},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{targetCluster},
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateDeployment(kubeClient, deployment)
		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	// Verifies a push-mode informer starts using a rotated member-cluster token.
	// Observing this naturally takes two ~10 min waits, which the test fakes to stay fast:
	//   1. The old token must become invalid. We revoke it (recreate the SA) instead of waiting
	//      for it to expire.
	//   2. The long-lived watch must reconnect. An open watch keeps the token it opened with,
	//      because the API server checks the token only at connect time, not per event. So if we
	//      don't force a reconnect, the watch keeps working on the old token and the test would
	//      pass even without the fix. We force it by restarting the member's node container.
	//
	//      NOTE: kubectl can't force this reconnect. The member API server is a static pod, so
	//      deleting its mirror pod leaves the process running and the connection never drops.
	//      Only a container restart drops it.
	ginkgo.It("informers recover after member cluster token rotates", func() {
		ginkgo.By("1. verify workload propagated and status collected (baseline)", func() {
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, deploymentNS, deploymentName,
				func(d *appsv1.Deployment) bool {
					return d.Status.ReadyReplicas > 0
				})
			klog.Infof("baseline: deployment %s/%s present on %s with ready replicas", deploymentNS, deploymentName, targetCluster)
		})

		cluster := &clusterv1alpha1.Cluster{}
		gomega.Expect(controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: targetCluster}, cluster)).Should(gomega.Succeed())
		gomega.Expect(cluster.Spec.SecretRef).ShouldNot(gomega.BeNil())
		secretNS := cluster.Spec.SecretRef.Namespace
		secretName := cluster.Spec.SecretRef.Name

		secret := &corev1.Secret{}
		gomega.Expect(controlPlaneClient.Get(context.TODO(), client.ObjectKey{Namespace: secretNS, Name: secretName}, secret)).Should(gomega.Succeed())
		saNamespace, saName := parseSAFromToken(string(secret.Data[clusterv1alpha1.SecretTokenKey]))
		gomega.Expect(saName).ShouldNot(gomega.BeEmpty(), "cluster token must be a parseable SA JWT")
		klog.Infof("cluster %s uses SA %s/%s on the member", targetCluster, saNamespace, saName)

		memberClient := framework.GetClusterClient(targetCluster)
		gomega.Expect(memberClient).ShouldNot(gomega.BeNil())

		ginkgo.By("2. rotate: revoke old token, mint new, write to Secret", func() {
			// Revoke first so the new token is bound to the recreated SA's UID.
			gomega.Expect(memberClient.CoreV1().ServiceAccounts(saNamespace).Delete(
				context.TODO(), saName, metav1.DeleteOptions{})).Should(gomega.Succeed())
			_, err := memberClient.CoreV1().ServiceAccounts(saNamespace).Create(
				context.TODO(), &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
					Name: saName, Namespace: saNamespace,
				}}, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			klog.Infof("revoked old tokens by recreating SA %s/%s", saNamespace, saName)

			tokenReq := &authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					ExpirationSeconds: ptr.To[int64](86400),
				},
			}
			tokenResp, err := memberClient.CoreV1().ServiceAccounts(saNamespace).CreateToken(
				context.TODO(), saName, tokenReq, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			newToken := tokenResp.Status.Token

			secret.Data[clusterv1alpha1.SecretTokenKey] = []byte(newToken)
			gomega.Expect(controlPlaneClient.Update(context.TODO(), secret)).Should(gomega.Succeed())
			klog.Infof("rotated token in Secret %s/%s", secretNS, secretName)

			// The apiserver caches SA tokens for ~12s before revocation takes effect.
			time.Sleep(revocationCacheFlushWait)
		})

		ginkgo.By("3. force watch reconnection (docker restart member node)", func() {
			containerName := targetCluster + "-control-plane"
			klog.Infof("restarting kind container %s to force watch reconnection", containerName)

			// Restart the container, not `kubectl delete pod`: the API server is a static pod, so
			// only a container restart drops the watch connection.
			cmd := kindexec.Command("docker", "restart", containerName)
			output, err := kindexec.CombinedOutputLines(cmd)
			if err != nil {
				klog.Warningf("docker restart failed (output: %v): %v — skipping e2e", output, err)
				ginkgo.Skip("cannot force watch reconnection via docker restart — run manual test instead")
			}
			klog.Infof("container %s restarted", containerName)

			gomega.Eventually(func() bool {
				c := &clusterv1alpha1.Cluster{}
				if err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: targetCluster}, c); err != nil {
					return false
				}
				for _, cond := range c.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == "True" {
						return true
					}
				}
				return false
			}, framework.PollTimeout, framework.PollInterval).Should(gomega.BeTrue(),
				"cluster must return to Ready after container restart")
		})

		ginkgo.By("4. verify short-lived path still works (cluster stays Ready)", func() {
			gomega.Consistently(func() bool {
				c := &clusterv1alpha1.Cluster{}
				if err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: targetCluster}, c); err != nil {
					return false
				}
				for _, cond := range c.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == "True" {
						return true
					}
				}
				return false
			}, clusterStableWindow, framework.PollInterval).Should(gomega.BeTrue(),
				"cluster must remain Ready (short-lived health check unaffected)")
		})

		ginkgo.By("5. verify long-lived path recovered (informer sees member changes)", func() {
			karmadaDep, err := kubeClient.AppsV1().Deployments(deploymentNS).Get(
				context.TODO(), deploymentName, metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			targetReplicas := *karmadaDep.Spec.Replicas + 2
			karmadaDep.Spec.Replicas = &targetReplicas
			_, err = kubeClient.AppsV1().Deployments(deploymentNS).Update(
				context.TODO(), karmadaDep, metav1.UpdateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			klog.Infof("scaled deployment to %d replicas", targetReplicas)

			gomega.Eventually(func() bool {
				dep, err := kubeClient.AppsV1().Deployments(deploymentNS).Get(
					context.TODO(), deploymentName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				klog.Infof("karmada collected readyReplicas=%d (target %d)",
					dep.Status.ReadyReplicas, targetReplicas)
				return dep.Status.ReadyReplicas == targetReplicas
			}, framework.PollTimeout, framework.PollInterval).Should(gomega.BeTrue(),
				"informer must observe scaled status after token rotation")
		})
	})
})

func parseSAFromToken(token string) (namespace, name string) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ""
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", ""
	}
	// sub format: "system:serviceaccount:<namespace>:<name>"
	segments := strings.Split(claims.Sub, ":")
	if len(segments) != 4 {
		return "", ""
	}
	return segments[2], segments[3]
}
