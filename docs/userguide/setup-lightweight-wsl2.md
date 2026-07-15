# Lightweight Karmada Setup on WSL2

## Overview

This guide explains how to run a lightweight local Karmada development environment on a low-resource Windows laptop using WSL2, Docker Desktop, Kind, and prebuilt Docker images.

The setup is optimized for contributors who want a faster and less resource-intensive workflow compared to the default Karmada local setup.

---

# Why Use a Lightweight Setup?

The default Karmada local setup can be resource intensive because it:

- Builds images from source
- Creates multiple Kind clusters
- Consumes significant RAM and CPU
- Takes longer to initialize

This lightweight setup improves contributor experience by:

- Using prebuilt Docker images
- Creating only one host cluster and one member cluster
- Reducing startup time
- Lowering memory and CPU usage
- Simplifying local debugging

---

# Environment Used

The setup in this guide was successfully verified using:

- Windows 11
- WSL2 Ubuntu
- Docker Desktop
- Kind
- kubectl
- Karmada lightweight setup

---

# Recommended WSL2 Optimization

Create the following file on Windows:

```ini
C:\Users\<username>\.wslconfig
```

Add:

```ini
[wsl2]
memory=4GB
processors=2
swap=2GB
```

After saving the file, restart WSL:

```powershell
wsl --shutdown
```

Then reopen your terminal.

---

# Prerequisites

Ensure the following tools are installed:

- WSL2
- Docker Desktop
- kubectl
- kind
- git

Verify installations:

```bash
kubectl version --client
kind version
docker --version
```

---

# Running the Lightweight Setup

Run the following command from the Karmada repository root:

```bash
bash hack/fast-local-up.sh
```

This setup:

- Uses prebuilt Docker images
- Avoids heavy source compilation
- Creates:
  - karmada-host cluster
  - member1 cluster
- Automatically joins member1 to the Karmada control plane

---

# Verify the Setup

## Configure kubeconfig

```bash
export KUBECONFIG=$HOME/.kube/karmada.config
```

---

## Switch Context

```bash
kubectl config use-context karmada-apiserver
```

---

## Verify Joined Clusters

```bash
kubectl get clusters
```

Expected output:

```text
NAME      VERSION   MODE   READY
member1   v1.35.0   Push   True
```

---

## Verify Workload Deployment

```bash
kubectl get deployment nginx-sample
```

Expected output:

```text
NAME           READY   UP-TO-DATE   AVAILABLE
nginx-sample   1/1     1            1
```

---

## Verify ResourceBinding

```bash
kubectl get rb
```

Expected output:

```text
NAME                      SCHEDULED   FULLYAPPLIED
nginx-sample-deployment   True        True
```

---

# Verify Pods on Member Cluster

The Karmada control plane stores workload templates, while actual Pods run on member clusters.

To verify running Pods on the member cluster:

```bash
export KUBECONFIG=$HOME/.kube/members.config
kubectl config use-context member1
kubectl get pods -A
```

Expected output includes:

```text
default     nginx-sample-xxxxx     1/1     Running
```

---

# Verify Docker Containers

Run:

```bash
docker ps
```

Expected containers:

```text
karmada-host-control-plane
member1-control-plane
```

---

# Troubleshooting

## PowerShell && Separator Issue

Problem:

```text
The token '&&' is not a valid statement separator in this version.
```

Cause:

Older PowerShell versions may not support Linux-style command chaining.

Solution:

Use:

```bash
wsl bash -c "command"
```

Instead of running chained Linux commands directly in PowerShell.

---

## Pods Not Visible on karmada-apiserver

Problem:

```text
kubectl get pods -A
No resources found
```

Explanation:

The Karmada control plane stores workload templates and policies.
Actual Pods run on member clusters.

Solution:

Switch to the member cluster kubeconfig:

```bash
export KUBECONFIG=$HOME/.kube/members.config
kubectl config use-context member1
kubectl get pods -A
```

---

## Missing member1 Context

Problem:

```text
error: no context exists with the name: "member1"
```

Cause:

The member cluster context exists inside:

```text
~/.kube/members.config
```

not:

```text
~/.kube/karmada.config
```

Solution:

Use the correct kubeconfig:

```bash
export KUBECONFIG=$HOME/.kube/members.config
```

---

## Slow Startup Time

Possible Causes:

- Building images from source
- Multiple cluster creation
- Low system resources

Solution:

Use:

- prebuilt Docker images
- minimal clusters
- lightweight setup script

---

## High RAM Usage

Solution:

Reduce WSL2 resources using:

```ini
[wsl2]
memory=4GB
processors=2
swap=2GB
```

Also:

- Close unused applications
- Disable unused Docker Desktop features
- Avoid running unnecessary clusters

---

## WSL Networking Considerations

Some setups may experience:

- port forwarding issues
- kubeconfig access issues
- Docker networking delays

Recommendations:

- Ensure Docker Desktop WSL integration is enabled
- Restart WSL if networking becomes unstable
- Restart Docker Desktop if clusters fail to connect

---

# Cleanup

To remove the Kind clusters:

```bash
kind delete clusters karmada-host member1
```

---

# Restart the Environment

To recreate the environment:

```bash
bash hack/fast-local-up.sh
```

---

# Useful Commands

## List Clusters

```bash
kubectl get clusters
```

---

## List Deployments

```bash
kubectl get deployment -A
```

---

## List ResourceBindings

```bash
kubectl get rb
```

---

## List Docker Containers

```bash
docker ps
```

---

## List Pods on Member Cluster

```bash
export KUBECONFIG=$HOME/.kube/members.config
kubectl config use-context member1
kubectl get pods -A
```

---

# Contributor Notes

This lightweight setup is useful for:

- local Karmada development
- debugging
- contributor onboarding
- low-resource laptops
- WSL2-based workflows
- faster testing iterations

It provides a significantly simpler environment for new contributors compared to the default multi-cluster local setup.



## Tested Environment

The lightweight setup was validated using the following environment:

* Windows 11
* WSL2 Ubuntu
* Docker Desktop
* 4 GB WSL2 memory limit
* 2 vCPUs
* 2 GB swap

### Cluster Topology

The validation used a minimal topology consisting of:

* 1 Karmada host cluster
* 1 member cluster

### Validation Results

The following functionality was successfully verified:

* Karmada control plane startup
* Member cluster registration
* Workload propagation
* ResourceBinding creation
* Deployment scheduling
* Successful workload execution on the member cluster

### Resource Observations

| Resource                      | Observed |
| ----------------------------- | -------- |
| WSL2 Memory Limit             | ~3.8 GiB |
| Memory Used After Startup     | ~2.2 GiB |
| Available Memory              | ~1.6 GiB |
| Swap Used                     | ~114 MiB |
| Host Cluster CPU Usage        | ~20%     |
| Member Cluster CPU Usage      | ~7%      |
| Host Cluster Memory           | ~1.4 GiB |
| Member Cluster Memory         | ~686 MiB |
| Host Cluster Writable Layer   | ~105 MB  |
| Member Cluster Writable Layer | ~3.6 MB  |

> **Note:** The lightweight setup was validated for contributor onboarding, local development, and debugging workflows. Overall Docker image cache and filesystem usage depend on the developer's local environment and are therefore not included as baseline requirements. This configuration has not been evaluated as a replacement for the full CI-equivalent end-to-end (e2e) test environment.
