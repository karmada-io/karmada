---
name: karmada-search
description: Help the user set up and query karmada-search for cross-cluster resource lookups. Use when the user mentions "search across clusters", ResourceRegistry, the search.karmada.io API, or asks how to find a pod/service across the whole fleet.
metadata:
  type: component
  loads:
    - knowledge/03-components.md
    - knowledge/00-overview.md
  status: scaffold
---

# karmada-search

> **Status:** scaffold. Full content (registry templates, query examples, troubleshooting)
> lands in mentorship weeks 8–9.

karmada-search is a *separate* aggregated apiserver in the Karmada control plane. It
indexes resources from member clusters and exposes `search.karmada.io/v1alpha1` so
operators can do `kubectl get pods -A` style queries against the fleet.

## Two artifacts a user typically asks about

1. **ResourceRegistry** — declares "search-index *these* kinds from *these* clusters".
2. **Search queries** — `kubectl get --raw "/apis/search.karmada.io/v1alpha1/search?…"`.

## Planned outline

- A `templates/resource-registry.minimal.yaml` for namespaced and cluster-scoped
  resources.
- A `knowledge/05-search-queries.md` cookbook with common queries.
- A troubleshooting case in `knowledge/02-troubleshooting.md` for "search returns
  empty / 404 / 500".
- A helper `scripts/render_search_query.py` that turns "find me all pods in
  namespace foo with label app=bar across all clusters" into the right raw URL.
