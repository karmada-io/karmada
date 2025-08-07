---
title: Proxy Support for Custom HTTP Source CRD Download Strategy in Karmada Operator
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2025-07-27

---

# Proxy Support for Custom HTTP Source CRD Download Strategy in Karmada Operator

## Summary

This proposal extends the custom HTTP source CRD download strategy for the Karmada operator by adding support for specifying a proxy server to be used when downloading the CRD tarball from an HTTP source. 
This enhancement aims to increase compatibility with restrictive corporate environments where cross-network traffic is mediated through a proxy.

## Motivation

In enterprise environments, especially those with strict network security policies, cross-network traffic often must be mediated through a proxy server. 
Currently, the Karmada operator's custom HTTP source CRD download strategy allows specifying an HTTP(S) URL as the source for the CRD tarball but does not support proxy configuration.
This limitation prevents the operator from functioning correctly in such restricted environments. By adding support for specifying a proxy, we can ensure that the Karmada operator 
is flexible and adaptable to diverse network configurations.

## Goals

- Enable proxy configuration for downloading CRD tarballs from custom HTTP/HTTPS sources.
- Maintain backward compatibility by keeping the proxy field optional.
- Ensure that the proxy configuration, if specified, is honored when downloading CRDs in both `Always` and `IfNotPresent` policies.

## Proposal

This proposal extends the `HTTPSource` type in the Karmada CRD specification to include an optional `Proxy` field. This field, when set, will specify the configuration of a proxy server to use when downloading the CRD tarball.

### API Changes

Update the `HTTPSource` type as follows:

```go
// HTTPSource specifies how to download the CRD tarball via either HTTP or HTTPS protocol.
type HTTPSource struct {
    // URL specifies the URL of the CRD tarball resource.
    URL string `json:"url,omitempty"`

    // Proxy specifies the configuration of a proxy server to use when downloading the CRD tarball.
    // When set, the operator will use the configuration to determine how to establish a connection to the proxy to fetch the tarball from the URL specified above.
    // This is useful in environments where direct access to the server hosting the CRD tarball is restricted and a proxy must be used to reach that server.
    // If a proxy configuration is not set, the operator will attempt to download the tarball directly from the URL specified above without using a proxy.
    // +optional
    Proxy *ProxyConfig `json:"proxy,omitempty"`
}

// ProxyConfig defines the configuration for a proxy server to use when downloading a CRD tarball.
type ProxyConfig struct {
    // ProxyURL specifies the HTTP/HTTPS proxy server URL to use when downloading the CRD tarball.
    // This is useful in environments where direct access to the server hosting the CRD tarball is restricted and a proxy must be used to reach that server.
    // The format should be a valid URL, e.g., "http://proxy.example.com:8080".
    // +kubebuilder:validation:Required
    ProxyURL string `json:"proxyURL"`
}
```

### Behavior

- If the `Proxy` field is set, the operator will configure the HTTP client to route requests through the specified proxy.
- If the `Proxy` field is not set, the HTTP client will behave as it does today (i.e., direct connection or system-level proxy settings).
- The proxy setting will apply only to the download of the CRD tarball. It will not affect other HTTP operations performed by the operator.

### Caching Behavior

This proposal does not alter the caching logic. The cache key for a given CRD tarball will continue to be derived from the URL alone. The proxy does not affect the identity of the downloaded content and therefore does not contribute to the cache key.

### Design Considerations

- **Extensibility**: The design allows for future support of authentication or SOCKS proxies if needed.

### Alternatives Considered

- Using environment variables to configure the proxy. This was rejected because it lacks the granularity and clarity of defining proxy settings on a per-resource basis within the CRD.

### Implementation Plan

- Extend the `HTTPSource` struct to include the `Proxy` field.
- Update the CRD schema and validation logic.
- Modify the download logic to honor the `Proxy` setting when set.

### Test Plan

- E2E tests in an environment requiring a proxy to ensure successful CRD downloads.
- E2E tests to verify that the behavior remains unchanged when the `Proxy` field is omitted.