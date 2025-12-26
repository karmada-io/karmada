# Init E2E Test Suite

This directory contains end-to-end (E2E) tests for the functionality of `karmadactl init`. These tests ensure that the 
initialization process of Karmada clusters works as expected.

It contains two categories of tests:
- Basic Tests: These tests create a simple Karmada control plane and make a propagation test to verify that the initialization 
  process is functioning correctly.
  path: test/e2e/suites/init/base_test.go
- Custom Tests: These tests create a Karmada control plane with custom configurations to validate that the custom 
initialization settings are applied correctly.

## Guide for Writing Tests

1. **Namespace Isolation**: Use the variable `testNamespace` to isolate the Karmada instance.

2. **Path Isolation**: Use the variable `karmadaDataPath` to isolate the data directory of the Karmada instance. When running the `init` command, specify the data paths using the following flags:
   ```go
   "--karmada-data", karmadaDataPath,
   "--karmada-pki", filepath.Join(karmadaDataPath, "pki"),
   "--etcd-data", filepath.Join(karmadaDataPath, "etcd-data"),
   ```

3. **Port Isolation**: Use the variable `karmadaAPIServerNodePort` to isolate the API server port. When running the `init` command, specify the port using the following flag:
   ```go
   "--port", karmadaAPIServerNodePort,
   ```
