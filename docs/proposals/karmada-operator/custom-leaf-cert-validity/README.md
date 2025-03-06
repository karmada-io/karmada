---
title: Support to Configure Leaf Certificate Validity Period in Karmada Operator
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2025-02-28

---

# Support to Configure Leaf Certificate Validity Period in Karmada Operator

## Summary

This proposal aims to extend the Karmada operator with support for configuring the validity period of leaf certificates (e.g., API server certificate). By allowing users to specify the validity period in days, this feature provides flexibility to align with organizational security policies and certificate management practices.

## Motivation

In scenarios where security policies require frequent rotation of certificates, the ability to configure the validity period of leaf certificates is essential. This feature ensures that the Karmada control plane can adhere to these policies by allowing users to specify a custom validity period for leaf certificates.

### Goals

- Allow users to specify the validity period of leaf certificates in days.
- Ensure that the configuration option is optional and defaults to the current behavior if not specified.
- Enable operators to align Karmada control plane PKI with organizational security policies.

### Non-Goals

- Change the default behavior of the Karmada operator when no custom validity period is provided.

## Proposal

The proposal introduces a new optional field, `LeafCertValidityDays`, in the `CustomCertificate` struct, where users can specify the validity period of leaf certificates in days.

### API Changes

```go
type CustomCertificate struct {
   // Other, existing fields omitted for brevity

   // LeafCertValidityDays specifies the validity period of leaf certificates (e.g., API Server certificate) in days.
   // If not specified, the default validity period of 1 year will be used.
   // +optional
   LeafCertValidityDays *int `json:"leafCertValidityDays,omitempty"`
}
```

### User Stories

#### Story 1
As a cloud infrastructure architect, I want to configure the validity period of leaf certificates to comply with my organization's security policies.


### Risks and Mitigations

1. *Incorrect Validity Period*: If the provided validity period is not a positive integer, the control plane setup may fail.

    - *Mitigation*: The operator will validate the provided validity period, ensuring it is a positive integer, and return detailed error messages if the configuration is incorrect.

2. *Backward Compatibility*: Introducing a custom validity period feature might impact users who do not need or configure this option.

    - *Mitigation*: This feature is fully optional; if no validity period is provided, the operator will default to the current behavior of 1 year, maintaining backward compatibility.

## Design Details

The `LeafCertValidityDays` field in `CustomCertificate` will allow users to specify the validity period of leaf certificates in days. During the reconciliation process, the Karmada operator will:

- Check if `CustomCertificate.LeafCertValidityDays` is set.
- If specified:
    - Use the provided validity period to derive the `NotAfter` field of the leaf certificate.
- If not specified:
    - Default to the current behavior of setting the `NotAfter` field to 1 year from the `NotBefore` field

This feature requires minimal changes to the reconciliation process and does not impact existing installations that do not specify a custom validity period.