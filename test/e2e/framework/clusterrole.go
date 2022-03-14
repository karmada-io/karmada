package framework

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateClusterrole create clusterrole.
func CreateClusterrole(client kubernetes.Interface, clusterrole *rbacv1.ClusterRole) {
	ginkgo.By(fmt.Sprintf("Creating ClusterRole(%s)", clusterrole.Name), func() {
		_, err := client.RbacV1().ClusterRoles().Create(context.TODO(), clusterrole, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// CreateClusterroleBinding create clusterrolebinding.
func CreateClusterroleBinding(client kubernetes.Interface, clusterrolebinding *rbacv1.ClusterRoleBinding) {
	ginkgo.By(fmt.Sprintf("Creating ClusterRoleBinding(%s)", clusterrolebinding.Name), func() {
		_, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterrolebinding, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitClusterrolebindingPresentOnCluster wait for Clusterrolebinding ready in cluster until timeout.
func WaitClusterrolebindingPresentOnCluster(client kubernetes.Interface, name string) {
	ginkgo.By(fmt.Sprintf("wait for clusterRoleBinding(%s) present ", name), func() {
		gomega.Eventually(func(g gomega.Gomega) (bool, error) {
			_, err := client.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			return true, nil
		}, pollTimeout, pollInterval).Should(gomega.Equal(true))
	})
}

// RemoveClusterrole delete clusterrole.
func RemoveClusterrole(client kubernetes.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Remove ClusterRole(%s)", name), func() {
		err := client.RbacV1().ClusterRoles().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveclusterroleBinding delete clusterrolebinding.
func RemoveclusterroleBinding(client kubernetes.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Remove ClusterRole(%s)", name), func() {
		err := client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// CreateMermberclusterSa create serviceaccount for member cluster.
func CreateMermberclusterSa(cluster string, serviceaccount *corev1.ServiceAccount) {
	ginkgo.By(fmt.Sprintf("Creating serviceaccount(%s) in membercluster (%s) in namespace %s", serviceaccount.Name, cluster, serviceaccount.Namespace), func() {
		clusterClient := GetClusterClient(cluster)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		_, err := clusterClient.CoreV1().ServiceAccounts(serviceaccount.Namespace).Create(context.TODO(), serviceaccount, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

}

// CreateKarmadaSa create serviceaccount for karmada.
func CreateKarmadaSa(client kubernetes.Interface, serviceaccount *corev1.ServiceAccount) {
	ginkgo.By(fmt.Sprintf("Creating serviceaccount(%s) in karmadaCluster in namespace %s", serviceaccount.Name, serviceaccount.Namespace), func() {
		_, err := client.CoreV1().ServiceAccounts(serviceaccount.Namespace).Create(context.TODO(), serviceaccount, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveMermberclusterSa delete serviceaccount for member cluster.
func RemoveMermberclusterSa(cluster, name, namespace string) {
	ginkgo.By(fmt.Sprintf("Remove serviceaccount(%s) in membercluster (%s)", name, cluster), func() {
		clusterClient := GetClusterClient(cluster)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		err := clusterClient.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveKarmadalusterSa delete serviceaccount for karmada cluster.
func RemoveKarmadalusterSa(client kubernetes.Interface, name, namespace string) {
	ginkgo.By(fmt.Sprintf("Remove serviceaccount(%s) in karmadaCluster", name), func() {
		err := client.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// CreateMemberclusterRole create clusterrole with config.
func CreateMemberclusterRole(cluster string, clusterrole *rbacv1.ClusterRole) {
	ginkgo.By(fmt.Sprintf("Creating member clusterrole(%s) in clustermember(%s)", clusterrole.Name, cluster), func() {
		clusterClient := GetClusterClient(cluster)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		_, err := clusterClient.RbacV1().ClusterRoles().Create(context.TODO(), clusterrole, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveMemberclusterRole delete member cluster clusterRole.
func RemoveMemberclusterRole(cluster string, clusterroleName string) {
	ginkgo.By(fmt.Sprintf("Remove clusterRole(%s) in member(%s) ", clusterroleName, cluster), func() {
		clusterClient := GetClusterClient(cluster)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		err := clusterClient.RbacV1().ClusterRoles().Delete(context.TODO(), clusterroleName, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// CreateMemberclusterRoleBinding create membercluster clusterrolebinding.
func CreateMemberclusterRoleBinding(cluster string, clusterrolebinding *rbacv1.ClusterRoleBinding) {
	ginkgo.By(fmt.Sprintf("Creating member clusterrolebinding(%s)", clusterrolebinding.Name), func() {
		clusterClient := GetClusterClient(cluster)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		_, err := clusterClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterrolebinding, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitClusterrolebindingPresentOnMemberCluster wait for Clusterrolebinding ready in member cluster until timeout.
func WaitClusterrolebindingPresentOnMemberCluster(cluster string, clusterroleBindName string) {
	ginkgo.By(fmt.Sprintf("wait for clusterRoleBinding(%s) present in member(%s) cluster ", clusterroleBindName, cluster), func() {
		clusterClient := GetClusterClient(cluster)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		gomega.Eventually(func(g gomega.Gomega) (bool, error) {
			_, err := clusterClient.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterroleBindName, metav1.GetOptions{})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			return true, nil
		}, pollTimeout, pollInterval).Should(gomega.Equal(true))
	})
}

// RemoveMemberclusterRoleBinding delete member cluster clusterRoleBinding.
func RemoveMemberclusterRoleBinding(cluster string, clusterroleBindName string) {
	ginkgo.By(fmt.Sprintf("Remove clusterRoleBinding(%s) in member(%s) ", clusterroleBindName, cluster), func() {
		clusterClient := GetClusterClient(cluster)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		err := clusterClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), clusterroleBindName, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitServiceaccountPresentOnCluster wait for sa ready on cluster untill timeout.
func WaitServiceaccountPresentOnCluster(client kubernetes.Interface, saname, namespace string) {
	ginkgo.By(fmt.Sprintf("wait for serviceaccount (%s) present in karmadacluster in namespace %s", saname, namespace), func() {
		gomega.Eventually(func(g gomega.Gomega) (bool, error) {
			_, err := client.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), saname, metav1.GetOptions{})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			return true, nil
		}, pollTimeout, pollInterval).Should(gomega.Equal(true))
	})
}

// WaitServiceaccountPresentOnMemberCluster wait for sa ready on targetmember cluster untill timeout.
func WaitServiceaccountPresentOnMemberCluster(cluster, saname, namespace string) {
	ginkgo.By(fmt.Sprintf("wait for serviceaccount (%s) present in member(%s) cluster in namespace %s ", saname, cluster, namespace), func() {
		clusterClient := GetClusterClient(cluster)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		gomega.Eventually(func(g gomega.Gomega) (bool, error) {
			_, err := clusterClient.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), saname, metav1.GetOptions{})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			return true, nil
		}, pollTimeout, pollInterval).Should(gomega.Equal(true))
	})
}

// WaitSecretPresentOnCluster wait for Clusterrolebinding ready until timeout.
func WaitSecretPresentOnCluster(client kubernetes.Interface, saname, namespace string) (secretName string) {
	ginkgo.By(fmt.Sprintf("wait for secret (%s) present in cluster in namespace %s", saname, namespace), func() {
		gomega.Eventually(func(g gomega.Gomega) (bool, error) {
			listSecret, err := client.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			sclist := []string{}
			for _, scName := range listSecret.Items {
				sclist = append(sclist, scName.Name)
			}
			for _, k := range sclist {
				secretName = k
			}
			return strings.HasPrefix(secretName, saname), nil
		}, pollTimeout, pollInterval).Should(gomega.Equal(true))
	})
	return
}

// GetSecretInfo get token/url of the karmada cluster.
func GetSecretInfo(client kubernetes.Interface, karmadaClient karmada.Interface, saname, namespace string) (token string, url string) {
	secretName := WaitSecretPresentOnCluster(client, saname, namespace)
	ta, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		fmt.Println(err.Error())
	}
	token = base64.StdEncoding.EncodeToString(ta.Data["token"])
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	url = config.Host
	if err != nil {
		panic(err.Error())
	}
	return token, url
}

// GetClusterNode use bearer token call karmada api.
func GetClusterNode(path string, client kubernetes.Interface, karmadaClient karmada.Interface, saname, namespace string) int {
	token, apiurl := GetSecretInfo(client, karmadaClient, saname, namespace)
	// changge toekn to decodestring
	to1, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		fmt.Println(err.Error())
	}
	t1 := string(to1)
	t := "Bearer " + t1
	url := apiurl + path
	// insecure the httprequest
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: tr}
	res, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	res.Header.Add("Authorization", t)
	resp, err := httpClient.Do(res)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
