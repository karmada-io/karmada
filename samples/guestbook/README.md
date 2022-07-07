# Propagate CRD application
The following steps demonstrating how to propagate a [Guestbook](https://book.kubebuilder.io/quick-start.html#create-a-project) which is defined by CRD.

Assume you are under the guestbook directory.
```bash
cd samples/guestbook
```
and set the KUBECONFIG environment with Karmada configuration.
```bash
export KUBECONFIG=${HOME}/.kube/karmada.config
```
1. Create Guestbook CRD in Karmada
```bash
kubectl apply -f guestbooks-crd.yaml 
``` 
The CRD should be applied to `karmada-apiserver`.

2. Create ClusterPropagationPolicy that will propagate Guestbook CRD to member cluster
```bash
kubectl apply -f guestbooks-clusterpropagationpolicy.yaml
```
The CRD will be propagated to member clusters according to the rules defined in ClusterPropagationPolicy
>Note: We can only use ClusterPropagationPolicy not PropagationPolicy here.
> Please refer to FAQ Difference between [PropagationPolicy and ClusterPropagationPolicy](https://github.com/karmada-io/karmada/blob/master/docs/frequently-asked-questions.md#what-is-the-difference-between-propagationpolicy-and-clusterpropagationpolicy)
> for more details.
3. Create a Guestbook named `guestbook-sample` in Karmada
```bash
kubectl apply -f guestbook.yaml
```
4. Create PropagationPolicy that will propagate `guestbook-sample` to member cluster
```bash 
kubectl apply -f guestbooks-propagationpolicy.yaml
```
5. Check the `guestbook-sample` status from Karmada
```bash
kubectl get guestbook -oyaml
```
6. Create OverridePolicy that will override the size field of guestbook-sample in member cluster
```bash
kubectl apply -f guestbooks-overridepolicy.yaml
```
7. Check the size field of `guestbook-sample` from member cluster
```bash
kubectl --kubeconfig=${HOME}/.kube/members.config config use-context member1
kubectl --kubeconfig=${HOME}/.kube/members.config get guestbooks -o yaml
```
If it works as expected, the `.spec.size` will be overwritten to `4`.