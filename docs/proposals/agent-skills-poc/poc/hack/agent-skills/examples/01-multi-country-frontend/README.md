# Example 01 — Multi-country frontend (Singapore, Indonesia, Germany, Brazil)

This is the canonical multi-country scenario from the upstream issue. It exercises:

- ordered-group placement (regional preference with failover)
- weighted replica division within a region
- per-country image-registry override (Indonesia uses a regional mirror)
- per-country ConfigMap field override (locale + currency)
- dependency propagation (ConfigMap rides along with the Deployment)

## Files

- `workload.yaml` — the Deployment + Service + ConfigMap the user wants to propagate
- `intent.json` — the structured intent passed to `generate_policy.py`
- `expected-policy.yaml` — the YAML the script should produce (used by tests)

## Replay

```
python3 ../../scripts/generate_policy.py --intent intent.json --out generated-policy.yaml
diff -u expected-policy.yaml generated-policy.yaml   # should be empty
python3 ../../scripts/audit_policy.py generated-policy.yaml --format text
```
