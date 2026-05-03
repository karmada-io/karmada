# Image Override Example

This example demonstrates how to use Karmada OverridePolicy to use different
container image registries per cluster, including image tag overrides for
canary deployments.

## Scenario

A microservice Deployment needs:
- Production images from `registry.example.com/prod` on member1 (production)
- Staging images from `registry.example.com/staging` on member2 (staging)
- Canary tag (`v2-canary`) instead of `latest` on member3 (canary)
- Specific container override in a multi-container Pod
