---
title: Enable karmadactl list image
authors:
  - "@tiansuo114"
reviewers:
  - "@liangyuanpeng"
  - "@zhzhuang-zju"
  - "@XiShanYongYe-Chang"

approvers:
  - "@liangyuanpeng"
  - "@zhzhuang-zju"
  - "@XiShanYongYe-Chang"

creation-date: 2024-07-29
---

# Enable karmadactl list image

## Summary
We want to introduce an `karmadactl init show-images` command. This command allows users to easily retrieve and print the list of container images required during the `karmadactl init` process. This feature is particularly useful for users who wish to download the images in advance, reducing deployment time, especially in offline or restricted environments.

## Motivation

Additionally, during the deployment process of `karmadactl init`, multiple container images are needed. Users may want to download the corresponding images in advance to save deployment time, but currently, there is no easy way for users to obtain the list of required container images.

### Goals
- Implement the `karmadactl init show-images` command to provide a list of container images.

### Non-Goals
- Add additional commands for karmadactl config, such as karmadactl config --init-config to add the ability to output default initialization config to the command
- Adds the ability to display the list of images required in karmadactl addon deployment by extending the cmd flag of karmadactl config image

## Proposal
We can extend the functionality of the `karmadactl init show-images` command to read the required image configuration from the config file by reusing the config file reading method.

### User Stories (Optional)

####  Pre-downloading Images

Ms. Li is a software engineer responsible for deploying Karmada in her company’s internal network. Due to network restrictions, she cannot retrieve the required images for Karmada deployment online. To download the images in advance, Ms. Li wants to use the `karmadactl init show-images` command to get a list of the necessary images.

After the feature is introduced, Ms. Li can execute the `karmadactl init show-images` command to retrieve the list of images required for Karmada deployment. She can also select the image repository based on her network conditions (such as her company’s internal image registry). By pre-downloading the images, Ms. Li ensures smooth deployment in an offline environment, reducing deployment time.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

## Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

### Adding New Commands

#### karmadactl init show-images

- description:Shows information about the images required for Karmada deployment.
- usage scope:Default image and configure the specified image
- image-related flag set:

| name                      | shorthand | default | usage                                                        |
| ------------------------- | --------- | ------- | ------------------------------------------------------------ |
| private-image-registry    | /         | ""      | Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios. |
| kube-image-mirror-country | /         | ""      | Country code of the kube image registry to be used. For Chinese mainland users, set it to cn |

- performance：
    1. When the `private-image-registry` and `kube-image-mirror-country` flags are not set, the command will return all required images for deployment, downloading them from the default image repository `registry.k8s.io`. This setup is ideal for users who have direct access to the internet and do not need to configure specific image sources.

       Command:

       ```bash
       karmadactl init show-images
       ```

       Output:

       ```
       registry.k8s.io/karmada-apiserver:v0.9.0
       registry.k8s.io/karmada-controller-manager:v0.9.0
       ...
       ```

    2. In some cases, users may need to download images from an internal image registry to avoid direct internet access. Users can specify the image source through the `private-image-registry` flag, and the command will return the image addresses from that private registry.

       Command:

       ```bash
       karmadactl init show-images --private-image-registry=registry.company.com
       ```

       Output:

       ```
       registry.company.com/karmada-apiserver:v0.9.0
       registry.company.com/karmada-controller-manager:v0.9.0
       ...
       ```

    3. For users in China, downloading images from local image sources can significantly speed up the process. When the `kube-image-mirror-country` is set to `cn`, the command will return image addresses from a Chinese mirror (e.g., Alibaba Cloud).

       Command:

       ```
       karmadactl init show-images --kube-image-mirror-country=cn
       ```

       Output:

       ```
       registry.cn-hangzhou.aliyuncs.com/google_containers/karmada-apiserver:v0.9.0
       registry.cn-hangzhou.aliyuncs.com/google_containers/karmada-controller-manager:v0.9.0
       ...
       ```

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

<!--
Note: This is a simplified version of kubernetes enhancement proposal template.
https://github.com/kubernetes/enhancements/tree/3317d4cb548c396a430d1c1ac6625226018adf6a/keps/NNNN-kep-template
-->
