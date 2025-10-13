# Testing Guide for Related Applications Failover Feature

This document provides comprehensive guidance on how to test the Related Applications Failover feature implemented in PR #6617.

## Overview

This feature allows related applications to be migrated together during application failover in Karmada. When a main application undergoes failover, all its related applications will also be migrated to the same target cluster, ensuring that interdependent applications remain co-located.

## Prerequisites

1. A Karmada control plane deployed
2. At least two member clusters registered with Karmada
3. `kubectl` and `karmadactl` installed and configured

## Testing Scenarios

### 1. Unit Tests

Run the unit tests for the application controllers:

```bash
# Test application controller
go test -v ./pkg/controllers/application/...

# Test application failover controller
go test -v ./pkg/controllers/applicationfailover/...
```

**Expected Results:**
- All tests should pass with 100% success rate
- Tests verify:
  - Basic failover functionality
  - Related applications migration
  - Graceful handling of missing related applications
  - Context propagation throughout the call chain

### 2. Integration Test: Basic Related Applications Failover

#### Step 1: Create the main application

```yaml
apiVersion: apps.karmada.io/v1alpha1
kind: Application
metadata:
  name: main-app
  namespace: default
spec:
  relatedApplications:
    - database-app
    - cache-app
```

Apply to Karmada:
```bash
kubectl apply -f main-app.yaml
```

#### Step 2: Create related applications

```yaml
apiVersion: apps.karmada.io/v1alpha1
kind: Application
metadata:
  name: database-app
  namespace: default
spec:
  relatedApplications: []
---
apiVersion: apps.karmada.io/v1alpha1
kind: Application
metadata:
  name: cache-app
  namespace: default
spec:
  relatedApplications: []
```

Apply to Karmada:
```bash
kubectl apply -f related-apps.yaml
```

#### Step 3: Trigger failover

Simulate a cluster failure or manually trigger failover for `main-app`:

```bash
# Check current cluster placement
kubectl get application main-app -o yaml

# Trigger failover (method depends on your Karmada setup)
# This could be done by:
# - Marking a cluster as unavailable
# - Manually updating the application's cluster placement
# - Simulating cluster health issues
```

#### Step 4: Verify related applications migration

```bash
# Check that main-app has been migrated
kubectl get application main-app -o yaml

# Verify that database-app has been migrated to the same cluster
kubectl get application database-app -o yaml

# Verify that cache-app has been migrated to the same cluster
kubectl get application cache-app -o yaml
```

**Expected Results:**
- Main application migrates to a healthy cluster
- Both `database-app` and `cache-app` migrate to the same cluster as `main-app`
- All applications are co-located in the target cluster

### 3. Integration Test: Missing Related Application

This test verifies graceful handling when a related application doesn't exist.

#### Step 1: Create main application with non-existent related app

```yaml
apiVersion: apps.karmada.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  relatedApplications:
    - existing-app
    - non-existent-app
```

#### Step 2: Create only one related application

```yaml
apiVersion: apps.karmada.io/v1alpha1
kind: Application
metadata:
  name: existing-app
  namespace: default
spec:
  relatedApplications: []
```

#### Step 3: Trigger failover

```bash
# Trigger failover for test-app
```

#### Step 4: Verify behavior

```bash
# Check logs for error messages about non-existent-app
kubectl logs -n karmada-system <controller-pod-name>

# Verify existing-app was still migrated
kubectl get application existing-app -o yaml
```

**Expected Results:**
- Failover proceeds successfully
- `existing-app` is migrated to the same cluster as `test-app`
- Error is logged for `non-existent-app` but doesn't block the failover
- Controller continues processing other related applications

### 4. Integration Test: Chained Related Applications

Test complex scenarios with multiple levels of related applications.

#### Step 1: Create application chain

```yaml
apiVersion: apps.karmada.io/v1alpha1
kind: Application
metadata:
  name: frontend-app
  namespace: default
spec:
  relatedApplications:
    - backend-app
---
apiVersion: apps.karmada.io/v1alpha1
kind: Application
metadata:
  name: backend-app
  namespace: default
spec:
  relatedApplications:
    - database-app
---
apiVersion: apps.karmada.io/v1alpha1
kind: Application
metadata:
  name: database-app
  namespace: default
spec:
  relatedApplications: []
```

#### Step 2: Trigger failover for frontend-app

#### Step 3: Verify all applications migrate together

**Expected Results:**
- `frontend-app` triggers migration
- `backend-app` migrates as a related application
- `database-app` (related to `backend-app`) should also be migrated if the controller handles transitive relationships

### 5. End-to-End Test

Run the complete E2E test suite:

```bash
# Run E2E tests
make test-e2e

# Run specific application failover tests
go test -v ./test/e2e/... -run TestApplicationFailover
```

**Expected Results:**
- All E2E tests pass successfully
- Application failover scenarios are validated in a real multi-cluster environment

## Verification Checklist

- [ ] Unit tests pass with 100% success rate
- [ ] Integration tests demonstrate successful related application migration
- [ ] Missing related applications are handled gracefully
- [ ] No panics or crashes occur during failover
- [ ] Logs show proper error handling and informational messages
- [ ] Context is properly propagated through the call chain
- [ ] All related applications end up in the same cluster as the main application
- [ ] E2E tests pass successfully

## Troubleshooting

### Issue: Related applications not migrating

**Check:**
1. Verify the related application names are correct
2. Ensure related applications exist in the same namespace
3. Check controller logs for errors:
   ```bash
   kubectl logs -n karmada-system <application-controller-pod> | grep -i error
   ```

### Issue: Failover not triggering

**Check:**
1. Verify cluster health status
2. Check application failover policy configuration
3. Review controller logs for reconciliation events

### Issue: Context deadline exceeded errors

**Check:**
1. Ensure context is properly passed through the call chain
2. Verify no `context.TODO()` usage in the codebase
3. Check for proper context timeout configuration

## Performance Considerations

When testing with many related applications:

1. **Batch Size**: The controller processes related applications sequentially
2. **Timeout**: Ensure sufficient timeout for large numbers of related applications
3. **Error Handling**: Failed migrations of individual related applications don't block others

## Logs to Monitor

Key log messages to watch for during testing:

```
# Successful migration
"Successfully migrated related application" name="<app-name>" namespace="<namespace>"

# Missing application
"Failed to get related application" name="<app-name>" error="<error-message>"

# Context issues
"context deadline exceeded" or "context canceled"
```

## CI/CD Integration

The feature is automatically tested in CI:

1. **Unit Tests**: Run on every commit
2. **Integration Tests**: Run on PR creation
3. **E2E Tests**: Run before merge
4. **Codegen Validation**: Ensures generated code is up to date

## Next Steps

After testing:

1. Review logs for any warnings or errors
2. Verify performance meets requirements
3. Document any edge cases discovered
4. Update this guide with new testing scenarios

## Support

For issues or questions:
- Create an issue in the Karmada repository
- Reference PR #6617 and issue #4613
- Include relevant logs and test results

