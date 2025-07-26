---
title: Custom CRD Download Strategy Support for Karmada Operator
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2024-07-05

---

# Custom CRD Download Strategy Support for Karmada Operator

## Summary

As a prerequisite for provisioning a new Karmada instance, the Karmada operator needs to ensure required CRDs are downloaded.
By default, it will attempt to download the CRDs tarball from a release artifact on GitHub. The URL for that release artifact is a well-known location
that is constructed based on the release version of the operator. This strategy makes the following assumptions, which may not always be correct:
- The constructed URL is accessible from the network where the operator is running. In an internal corporate environment with network policies that,
by default, prevent access to resources on the internet, this may not be true.
- The version of each Karmada instance managed by the operator is identical to that of the operator's release version. That may not always be true. When creating a new Karmada instance,
it's possible to explicitly specify the version of that instance's control plane components. Furthermore, the operator may have to manage multiple Karmada instances of heterogeneous versions.


## Motivation

The motive of this document is to propose a custom CRD downloading strategy to address the assumptions made by the Karmada operator which may not always be correct.


### Goals

The goal of this proposal is to provide the ability to specify a custom CRD downloading strategy for how the operator will download the CRDs tarball for a Karmada instance.
Furthermore, this strategy will also ensure the version of the CRDs used to provision a new Karmada instance is based on the intended version of that instance, not the release version of the operator.

## Proposal

Currently, when provisioning a new Karmada instance, as a prerequisite, the Karmada operator will ensure that required CRDs are downloaded. By default, the CRDs tarball is
downloaded from a well-known location on GitHub based on the release version of the operator. As aforementioned, this is based on the assumption that the version of the Karmada instance 
is identical to that of the operator's release version, which may not always be true. When the version of the Karmada control plane components is not specified, the default behavior is to assume
those components should have the same version as the operator's release version. In that scenario, the assumption is justified and holds true. However, when creating a new Karmada instance, it's possible to explicitly
specify the version of the control plane components. In that scenario, this assumption breaks, and we encounter the risk of the operator installing the wrong CRDs for that Karmada instance.

The default behavior is sensible and will be retained. However, it should not be the behavior when the intent is to create a new Karmada instance with control plane components of a different version than that of the operator's release version. 
To support that scenario, we require the ability to explicitly specify, as part of the Karmada resource, how the operator should download the CRDs tarball for that Karmada instance.

In addition to making sure required CRDs are downloaded, the Karmada operator also employs a caching strategy to avoid re-downloading CRDs it has already downloaded. It's a cache on the local file system.
Downloaded CRDs are stored at a well-known location on the local file system based the release version of the Karmada operator. When provisioning a new Karmada instance, before attempting to download the required CRDs, the operator will fist check if
they already exist in the cache. To support custom downloading strategies, we require this caching strategy be extended to be more generic.

## Design Details

To support custom CRD download strategies, we need to add the new `CRDTarball` field to the Karmada resource spec:
```go
// CRDDownloadPolicy specifies a policy for how the operator will download the Karmada CRD tarball
type CRDDownloadPolicy string

const (
	// DownloadAlways instructs the Karmada operator to always download the CRD tarball from a remote location.
	DownloadAlways CRDDownloadPolicy = "Always"

	// DownloadIfNotPresent instructs the Karmada operator to download the CRDs tarball from a remote location only if it is not yet present in the local cache.
	DownloadIfNotPresent CRDDownloadPolicy = "IfNotPresent"
)

// HTTPSource specifies how to download the CRD tarball via either HTTP or HTTPS protocol.
type HTTPSource struct {
	// URL specifies the URL of the CRD tarball resource.
	URL string `json:"url,omitempty"`
}

// CRDTarball specifies the source from which the Karmada CRD tarball should be downloaded, along with the download policy to use.
type CRDTarball struct {
	// HTTPSource specifies how to download the CRD tarball via either HTTP or HTTPS protocol.
	// +optional
	HTTPSource *HTTPSource `json:"httpSource,omitempty"`

	// CRDDownloadPolicy specifies a policy that should be used to download the CRD tarball.
	// Valid values are "Always" and "IfNotPresent".
	// Defaults to "IfNotPresent".
	// +kubebuilder:validation:Enum=Always;IfNotPresent
	// +kubebuilder:default=IfNotPresent
	// +optional
	CRDDownloadPolicy *CRDDownloadPolicy `json:"crdDownloadPolicy,omitempty"`
}

// KarmadaSpec is the specification of the desired behavior of the Karmada.
type KarmadaSpec struct {
	// Other fields

	// CRDTarball specifies the source from which the Karmada CRD tarball should be downloaded, along with the download policy to use.
	// If not set, the operator will download the tarball from a GitHub release.
	// By default, it will download the tarball of the same version as the operator itself.
	// For instance, if the operator's version is v1.10.0, the tarball will be downloaded from the following location:
	// https://github.com/karmada-io/karmada/releases/download/v1.10.0/crds.tar.gz
	// By default, the operator will only attempt to download the tarball if it's not yet present in the local cache.
	// +optional
	CRDTarball *CRDTarball `json:"crdTarball,omitempty"`
}
```

The default downloading strategy is sensible and will be retained. As such, this new field is optional. If the intent is to provision a new Karmada instance with control plane components of the same version as the operator's
release version, it should be left unspecified as part of the Karmada resource. Otherwise, it should be explicitly set to instruct the operator how to download the CRD tarball.

Today, an `HTTPSource` can be specified to download the CRD tarball via either HTTP or HTTPS protocols. However, this design is extensible, so in the future, if needed, it's possible to extend it to support other sources.

Currently, the cache for previously downloaded CRDs lives on the local file system at `/var/lib/karmada/<operator-release-version>`. To support custom CRD download strategies, we need to a new directory structure for the cache.
The Karmada operator can potentially be managing the lifecycle of multiple Karmada instances of heterogeneous versions, and as such, may have to download CRD tarballs from multiple sources. We need to cache the CRDs for each source.
Given some source, it's cache entry will live on the local file system at path `/var/lib/karmada/cache/<cache-key>`. The cache key will be the computed SHA-256 hash of some property derived from that source. In the case of `HTTPSource`, it will be the hash of the URL.