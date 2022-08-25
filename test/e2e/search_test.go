package e2e

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[karmada-search] karmada search testing", ginkgo.Ordered, func() {
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

	ginkgo.BeforeAll(func() {
		// get clusters' name
		pushModeClusters := framework.ClusterNamesWithSyncMode(clusterv1alpha1.Push)
		ginkgo.By(fmt.Sprintf("get %v clusters in push mode", len(pushModeClusters)))
		gomega.Expect(len(pushModeClusters) >= 2).Should(gomega.BeTrue())
		pushModeClusters = pushModeClusters[:2]
		sort.Strings(pushModeClusters)
		member1, member2 = pushModeClusters[0], pushModeClusters[1]
		ginkgo.By(fmt.Sprintf("test on %v and %v", member1, member2))

		// clean ResourceRegistries before test
		gomega.Expect(karmadaClient.SearchV1alpha1().ResourceRegistries().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})).Should(gomega.Succeed())
	})

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

	ginkgo.Describe("karmada proxy testing", ginkgo.Ordered, func() {
		var (
			m1Client, m2Client, proxyClient    kubernetes.Interface
			m1Dynamic, m2Dynamic, proxyDynamic dynamic.Interface
			nodeGVR                            = corev1.SchemeGroupVersion.WithResource("nodes")
		)

		ginkgo.BeforeAll(func() {
			m1Dynamic = framework.GetClusterDynamicClient(member1)
			gomega.Expect(m1Dynamic).ShouldNot(gomega.BeNil())
			m2Dynamic = framework.GetClusterDynamicClient(member2)
			gomega.Expect(m2Dynamic).ShouldNot(gomega.BeNil())

			m1Client = framework.GetClusterClient(member1)
			gomega.Expect(m1Client).ShouldNot(gomega.BeNil())
			m2Client = framework.GetClusterClient(member2)
			gomega.Expect(m2Client).ShouldNot(gomega.BeNil())

			proxyConfig := *restConfig
			proxyConfig.Host += "/apis/search.karmada.io/v1alpha1/proxying/karmada/proxy"
			proxyClient = kubernetes.NewForConfigOrDie(&proxyConfig)
			proxyDynamic = dynamic.NewForConfigOrDie(&proxyConfig)
		})

		ginkgo.Describe("resourceRegistry testings", func() {
			var (
				rr1, rr2           *searchv1alpha1.ResourceRegistry
				m1Deploy, m2Deploy *appsv1.Deployment
			)

			ginkgo.BeforeAll(func() {
				rr1 = &searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rr-" + rand.String(RandomStrLength),
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
				rr2 = &searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rr-" + rand.String(RandomStrLength),
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
							{
								APIVersion: "v1",
								Kind:       "Pod",
							},
						},
					},
				}

				m1Deploy = testhelper.NewDeployment(testNamespace, "proxy-m1-"+rand.String(RandomStrLength))
				framework.CreateDeployment(m1Client, m1Deploy)

				m2Deploy = testhelper.NewDeployment(testNamespace, "proxy-m2-"+rand.String(RandomStrLength))
				framework.CreateDeployment(m2Client, m2Deploy)
			})

			ginkgo.AfterAll(func() {
				err := karmadaClient.SearchV1alpha1().ResourceRegistries().Delete(context.TODO(), rr1.Name, metav1.DeleteOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
				err = karmadaClient.SearchV1alpha1().ResourceRegistries().Delete(context.TODO(), rr2.Name, metav1.DeleteOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}

				framework.RemoveDeployment(m1Client, testNamespace, m1Deploy.Name)
				framework.RemoveDeployment(m2Client, testNamespace, m2Deploy.Name)
			})

			ginkgo.It("create, update, delete resourceRegistry", func() {
				ginkgo.By("no resourceRegistries testings", func() {
					ginkgo.By("should not list pods", func() {
						gomega.Eventually(func(g gomega.Gomega) {
							list, err := proxyClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{})
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							g.Expect(list.Items).Should(gomega.BeEmpty())
						}, time.Second*10).Should(gomega.Succeed())
					})

					ginkgo.By("should not list nodes", func() {
						gomega.Eventually(func(g gomega.Gomega) {
							list, err := proxyClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							g.Expect(list.Items).Should(gomega.BeEmpty())
						}, time.Second*10).Should(gomega.Succeed())
					})
				})

				ginkgo.By("create resourceRegistry rr1 for nodes", func() {
					framework.CreateResourceRegistry(karmadaClient, rr1)

					ginkgo.By("should not list pods", func() {
						gomega.Eventually(func(g gomega.Gomega) {
							list, err := proxyClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{})
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							g.Expect(list.Items).Should(gomega.BeEmpty())
						}, time.Second*10).Should(gomega.Succeed())
					})

					ginkgo.By("should list nodes", func() {
						gomega.Eventually(func(g gomega.Gomega) {
							list, err := proxyClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							g.Expect(list.Items).ShouldNot(gomega.BeEmpty())
						}, time.Second*10).Should(gomega.Succeed())
					})
				})

				ginkgo.By("create resourceRegistry rr2 for pods, nodes", func() {
					framework.CreateResourceRegistry(karmadaClient, rr2)

					ginkgo.By("should list pods", func() {
						gomega.Eventually(func(g gomega.Gomega) {
							list, err := proxyClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{})
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							g.Expect(list.Items).ShouldNot(gomega.BeEmpty())
						}, time.Second*10).Should(gomega.Succeed())
					})

					ginkgo.By("should list nodes", func() {
						gomega.Eventually(func(g gomega.Gomega) {
							list, err := proxyClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							g.Expect(list.Items).ShouldNot(gomega.BeEmpty())
						}, time.Second*10).Should(gomega.Succeed())
					})
				})

				ginkgo.By("delete resourceRegistries rr1 and rr2", func() {
					framework.RemoveResourceRegistry(karmadaClient, rr1.Name)
					framework.RemoveResourceRegistry(karmadaClient, rr2.Name)

					ginkgo.By("should not list pods", func() {
						gomega.Eventually(func(g gomega.Gomega) {
							list, err := proxyClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{})
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							g.Expect(list.Items).Should(gomega.BeEmpty())
						}, time.Second*10).Should(gomega.Succeed())
					})

					ginkgo.By("should not list nodes", func() {
						gomega.Eventually(func(g gomega.Gomega) {
							list, err := proxyClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							g.Expect(list.Items).Should(gomega.BeEmpty())
						}, time.Second*10).Should(gomega.Succeed())
					})
				})
			})
		})
		ginkgo.Describe("access resources testings", func() {
			ginkgo.Context("caching nodes", func() {
				var (
					rr *searchv1alpha1.ResourceRegistry
				)

				ginkgo.BeforeAll(func() {
					rr = &searchv1alpha1.ResourceRegistry{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rr-" + rand.String(RandomStrLength),
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
					framework.RemoveResourceRegistry(karmadaClient, rr.Name)
				})

				ginkgo.It("could list nodes", func() {
					fromM1 := framework.GetResourceNames(m1Dynamic.Resource(nodeGVR))
					ginkgo.By("list nodes from member1: " + strings.Join(fromM1.List(), ","))
					fromM2 := framework.GetResourceNames(m2Dynamic.Resource(nodeGVR))
					ginkgo.By("list nodes from member2: " + strings.Join(fromM2.List(), ","))
					fromMembers := sets.NewString().Union(fromM1).Union(fromM2)

					gomega.Eventually(func(g gomega.Gomega) {
						fromProxy := framework.GetResourceNames(proxyDynamic.Resource(nodeGVR))
						g.Expect(fromProxy).Should(gomega.Equal(fromMembers))
					}, time.Second*10).Should(gomega.Succeed())
				})

				ginkgo.It("could chunk list nodes", func() {
					fromM1, err := m1Client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					ginkgo.By(fmt.Sprintf("list %v nodes from member1", len(fromM1.Items)))

					fromM2, err := m2Client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					total := len(fromM1.Items) + len(fromM2.Items)
					ginkgo.By(fmt.Sprintf("list %v nodes from member2", len(fromM2.Items)))

					list1, err := proxyClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{Limit: 1})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(list1.Items).Should(gomega.HaveLen(1))

					if list1.Continue == "" {
						ginkgo.By("No more nodes remains")
						gomega.Expect(list1.Items).Should(gomega.HaveLen(total))
						ginkgo.Skip("No more nodes remains")
					}

					list2, err := proxyClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
						Limit:    999999999999,
						Continue: list1.Continue,
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(list2.Items).Should(gomega.HaveLen(total - len(list1.Items)))
				})

				ginkgo.It("could list & watch nodes", func() {
					listObj, err := proxyClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					watcher, err := proxyClient.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{ResourceVersion: listObj.ResourceVersion})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					defer watcher.Stop()

					testNode := framework.GetAnyResourceOrFail(m1Dynamic.Resource(nodeGVR))

					anno := "proxy-ann-" + rand.String(RandomStrLength)
					data := []byte(`{"metadata": {"annotations": {"` + anno + `": "true"}}}`)
					_, err = m1Client.CoreV1().Nodes().Patch(context.TODO(), testNode.GetName(), types.StrategicMergePatchType, data, metav1.PatchOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					gomega.Eventually(func() bool {
						event := <-watcher.ResultChan()
						o, ok := event.Object.(*corev1.Node)
						if !ok {
							return false
						}
						return o.UID == testNode.GetUID() && metav1.HasAnnotation(o.ObjectMeta, anno)
					}, time.Second*10, 0).Should(gomega.BeTrue())
				})

				ginkgo.It("could path nodes", func() {
					testObject := framework.GetAnyResourceOrFail(m1Dynamic.Resource(nodeGVR))

					anno := "proxy-ann-" + rand.String(RandomStrLength)
					data := []byte(`{"metadata": {"annotations": {"` + anno + `": "true"}}}`)
					_, err := proxyClient.CoreV1().Nodes().Patch(context.TODO(), testObject.GetName(), types.StrategicMergePatchType, data, metav1.PatchOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					testPod, err := m1Client.CoreV1().Nodes().Get(context.TODO(), testObject.GetName(), metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(testPod.Annotations).Should(gomega.HaveKey(anno))
				})
			})
		})
	})
})
