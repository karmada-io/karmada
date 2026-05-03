# Multi-Country Cluster Setup

This example demonstrates a Karmada setup for a global application deployed across
Singapore, Indonesia, Germany, and Brazil with country-aware placement, failover,
and country-specific overrides.

## Scenario

A web application needs to run in four countries:
- **Singapore** (asia-southeast1-sg): Primary for Southeast Asia
- **Indonesia** (asia-southeast2-id): Secondary for Southeast Asia
- **Germany** (europe-west1-de): Primary for Europe
- **Brazil** (southamerica-east1-br): Primary for South America

Requirements:
- Deploy to all four clusters
- Country-specific labels for routing to CDN endpoints
- Different replica counts per region based on traffic
- Region-aware failover: if SG fails, overflow to ID; if DE fails, no immediate overflow
- Production in SG/DE, staging in ID/BR
