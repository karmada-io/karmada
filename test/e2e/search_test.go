package e2e

import (
	"context"
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[karmada-search] karmada search testing", func() {
	var member1 = "member1"
	var member2 = "member2"

	var member1NodeName = "member1-control-plane"
	var member2NodeName = "member2-control-plane"
	var member1PodName = "etcd-member1-control-plane"
	var member2PodName = "etcd-member2-control-plane"
	var existsDeploymentName = "coredns"
	var existsServiceName = "kubernetes"
	var existsDaemonsetName = "kube-proxy"
	var existsClusterRoleName = "cluster-admin"
	var existsClusterRoleBindingName = "cluster-admin"

	var pathPrefix = "/apis/search.karmada.io/v1alpha1/search/cache/"
	var pathAllServices = pathPrefix + "api/v1/services"
	var pathAllNodes = pathPrefix + "api/v1/nodes"
	var pathAllPods = pathPrefix + "api/v1/pods"
	var pathAllDeployments = pathPrefix + "apis/apps/v1/deployments"
	var pathAllDaemonsets = pathPrefix + "apis/apps/v1/daemonsets"
	var pathAllClusterRoles = pathPrefix + "apis/rbac.authorization.k8s.io/v1/clusterroles"
	var pathAllClusterRoleBindings = pathPrefix + "apis/rbac.authorization.k8s.io/v1/clusterrolebindings"
	var pathNSDeploymentsFmt = pathPrefix + "apis/apps/v1/namespaces/%s/deployments"
	// var pathWithLabel = pathPrefix + "apis/apps/v1/namespaces/kube-system/deployments?labelSelector=k8s-app=kube-dns"

	// var pollTimeout = 30 * time.Second
	var searchObject = func(path, target string, exists bool) {
		gomega.Eventually(func() bool {
			res := karmadaClient.SearchV1alpha1().RESTClient().Get().AbsPath(path).Do(context.TODO())
			gomega.Expect(res.Error()).ShouldNot(gomega.HaveOccurred())
			raw, err := res.Raw()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			return strings.Contains(string(raw), target)
		}, pollTimeout, pollInterval).Should(gomega.Equal(exists))
	}

	// use service as search object
	ginkgo.Describe("no ResourceRegistry testings", func() {
		ginkgo.It("[Service] should be not searchable", func() {
			searchObject(pathAllServices, existsServiceName, false)
		})
	})

	// use deployment, node, pod as search object
	ginkgo.Describe("create ResourceRegistry testings", func() {
		// use deployment as search object
		ginkgo.Context("caching cluster member1", ginkgo.Ordered, func() {
			var rrName string
			var rr *searchv1alpha1.ResourceRegistry
			var m1DmName, m2DmName string
			var m1Dm, m2Dm *appsv1.Deployment
			var member1Client, member2Client kubernetes.Interface

			ginkgo.BeforeAll(func() {
				member1Client = framework.GetClusterClient(member1)
				gomega.Expect(member1Client).ShouldNot(gomega.BeNil())
				m1DmName = "rr-member1-deployment-" + rand.String(RandomStrLength)
				m1Dm = testhelper.NewDeployment(testNamespace, m1DmName)
				framework.CreateDeployment(member1Client, m1Dm)

				member2Client = framework.GetClusterClient(member2)
				gomega.Expect(member2Client).ShouldNot(gomega.BeNil())
				m2DmName = "rr-member2-deployment-" + rand.String(RandomStrLength)
				m2Dm = testhelper.NewDeployment(testNamespace, m2DmName)
				framework.CreateDeployment(member2Client, m2Dm)

				rrName = "rr-" + rand.String(RandomStrLength)
				rr = &searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{
						Name: rrName,
					},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{member1},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
							},
						},
					},
				}
				framework.CreateResourceRegistry(karmadaClient, rr)
			})

			ginkgo.AfterAll(func() {
				framework.RemoveDeployment(member1Client, testNamespace, m1DmName)
				framework.RemoveDeployment(member2Client, testNamespace, m2DmName)
				framework.RemoveResourceRegistry(karmadaClient, rrName)
			})

			ginkgo.It("[member1 deployments] should be searchable", func() {
				searchObject(pathAllDeployments, existsDeploymentName, true)
			})

			ginkgo.It("[member2 deployments] should be not searchable", func() {
				searchObject(pathAllDeployments, m2DmName, false)
			})

			ginkgo.It("[member1 deployments namespace] should be searchable", func() {
				searchObject(fmt.Sprintf(pathNSDeploymentsFmt, testNamespace), m1DmName, true)
			})

			ginkgo.It("[memeber2 deployments namespace] should be not searchable", func() {
				searchObject(fmt.Sprintf(pathNSDeploymentsFmt, testNamespace), m2DmName, false)
			})

			// ginkgo.It("[deployments label] should be searchable", func() {
			// searchObject(pathWithLabel, existsDeploymentName, true)
			// })
		})

		// use node as search object
		ginkgo.Context("caching cluster member1 & member2", ginkgo.Ordered, func() {
			var rrName string
			var rr *searchv1alpha1.ResourceRegistry

			ginkgo.BeforeAll(func() {
				rrName = "rr-" + rand.String(RandomStrLength)
				rr = &searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{
						Name: rrName,
					},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{member1, member2},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							{
								APIVersion: "v1",
								Kind:       "Node",
							},
						},
					},
				}
				framework.CreateResourceRegistry(karmadaClient, rr)
			})

			ginkgo.AfterAll(func() {
				framework.RemoveResourceRegistry(karmadaClient, rrName)
			})

			ginkgo.It("[member1 nodes] should be searchable", func() {
				searchObject(pathAllNodes, member1NodeName, true)
			})

			ginkgo.It("[member2 nodes] should be searchable", func() {
				searchObject(pathAllNodes, member2NodeName, true)
			})
		})

		// use pod as search object
		ginkgo.Context("add two resourceRegistry", ginkgo.Ordered, func() {
			var rrName string
			var rr *searchv1alpha1.ResourceRegistry
			var rr2Name string
			var rr2 *searchv1alpha1.ResourceRegistry

			ginkgo.BeforeAll(func() {
				rrName = "rr-" + rand.String(RandomStrLength)
				rr = &searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{
						Name: rrName,
					},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{member1},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							{
								APIVersion: "v1",
								Kind:       "Pod",
							},
						},
					},
				}
				framework.CreateResourceRegistry(karmadaClient, rr)

				rr2Name = "rr-" + rand.String(RandomStrLength)
				rr2 = &searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{
						Name: rr2Name,
					},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{member2},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							{
								APIVersion: "v1",
								Kind:       "Pod",
							},
						},
					},
				}
				framework.CreateResourceRegistry(karmadaClient, rr2)
			})

			ginkgo.AfterAll(func() {
				framework.RemoveResourceRegistry(karmadaClient, rrName)
				framework.RemoveResourceRegistry(karmadaClient, rr2Name)
			})

			ginkgo.It("[member1 pods] should be searchable", func() {
				searchObject(pathAllPods, member1PodName, true)
			})

			ginkgo.It("[member2 pods] should be searchable", func() {
				searchObject(pathAllPods, member2PodName, true)
			})
		})
	})

	// use clusterrole, clusterrolebinding as search object
	ginkgo.Describe("update resourceRegistry", ginkgo.Ordered, func() {
		var rrName string
		var rr *searchv1alpha1.ResourceRegistry

		ginkgo.BeforeAll(func() {
			rrName = "rr-" + rand.String(RandomStrLength)
			rr = &searchv1alpha1.ResourceRegistry{
				Spec: searchv1alpha1.ResourceRegistrySpec{
					TargetCluster: policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{member1},
					},
					ResourceSelectors: []searchv1alpha1.ResourceSelector{
						{
							APIVersion: "rbac.authorization.k8s.io/v1",
							Kind:       "ClusterRole",
						},
					},
				},
			}
			rr.ObjectMeta.Name = rrName
			framework.CreateResourceRegistry(karmadaClient, rr)
		})

		ginkgo.AfterAll(func() {
			framework.RemoveResourceRegistry(karmadaClient, rrName)
		})

		ginkgo.It("[clusterrole] should be searchable", func() {
			searchObject(pathAllClusterRoles, existsClusterRoleName, true)
		})

		ginkgo.It("[clusterrolebinding] should not be searchable", func() {
			searchObject(pathAllClusterRoleBindings, existsClusterRoleBindingName, false)
		})

		ginkgo.It("[clusterrolebinding] should be searchable", func() {
			ginkgo.By("update resourceRegistry, add clusterrolebinding")
			rr.Spec.ResourceSelectors = []searchv1alpha1.ResourceSelector{
				{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				},
				{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
			}
			framework.UpdateResourceRegistry(karmadaClient, rr)
			searchObject(pathAllClusterRoleBindings, existsClusterRoleBindingName, true)
		})
	})

	// use daemonset as search object
	ginkgo.Describe("delete resourceRegistry", ginkgo.Ordered, func() {
		var rrName string
		var rr *searchv1alpha1.ResourceRegistry

		ginkgo.It("[daemonset] should be searchable", func() {
			ginkgo.By("create resourceRegistry, add daemonset")
			rrName = "rr-" + rand.String(RandomStrLength)
			rr = &searchv1alpha1.ResourceRegistry{
				Spec: searchv1alpha1.ResourceRegistrySpec{
					TargetCluster: policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{member1},
					},
					ResourceSelectors: []searchv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "DaemonSet",
						},
					},
				},
			}
			rr.ObjectMeta.Name = rrName
			framework.CreateResourceRegistry(karmadaClient, rr)
			searchObject(pathAllDaemonsets, existsDaemonsetName, true)
		})

		ginkgo.It("[daemonset] should not be searchable", func() {
			ginkgo.By("delete resourceRegistry")
			framework.RemoveResourceRegistry(karmadaClient, rrName)
			searchObject(pathAllDaemonsets, existsDaemonsetName, false)
		})
	})

	ginkgo.Describe("backend store testing", ginkgo.Ordered, func() {
	})
})
