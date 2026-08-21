# WSL2 Low-Memory Optimization Guide

This guide provides optimization steps for contributors running Karmada on Windows via WSL2, particularly those with low-resource environments (e.g., 8GB or 16GB RAM). Running multiple Kind clusters can consume significant resources. Depending on the WSL version and configuration, the default resource allocation can lead to high memory consumption and potential system freezes on constrained hosts.

## 1. `.wslconfig` Memory Tuning Notes

To restrict the memory and CPU allocated to WSL2, you should configure a `.wslconfig` file in your Windows user profile directory (e.g., `C:\Users\<YourUsername>\.wslconfig`).

### `.wslconfig` Example

```ini
[wsl2]
; Limits VM memory to use no more than 4 GB
memory=4GB
; Limits VM to use no more than 2 logical processors
processors=2
; Sets swap storage to help with memory pressure
swap=2GB
```

## 2. Restart Workflow

After modifying `.wslconfig`, you must fully shut down the WSL2 virtual machine for the changes to take effect.

Open a PowerShell or Windows Command Prompt terminal and run:

```powershell
# Shutdown all running WSL instances
wsl --shutdown

# Start WSL again (simply opening your WSL terminal or running 'wsl' will start it)
wsl
```

## 3. Verification Commands

Once WSL2 has restarted, verify that the limits are successfully applied before running the Karmada setup scripts.

From inside your WSL2 Ubuntu terminal, run:

```bash
# Check available memory and swap
free -h

# Check CPU cores
nproc
```

The output of `free -h` should show total memory close to the 4GB limit defined in your `.wslconfig`.

## 4. Troubleshooting

* **Slow Startup or Pulling Images:** After applying these limits, pulling Docker images may take slightly longer, but your host system will remain responsive.
* **Memory Pressure & Evictions:** If you notice pods being evicted or `OOMKilled` inside the Kind clusters, consider increasing the `swap` value or bumping `memory` to `6GB` if your host has 16GB total RAM.
* **Docker Desktop Considerations:** If you are using Docker Desktop instead of native Docker inside WSL2, ensure that Docker Desktop is configured to use the WSL2 backend. The limits set in `.wslconfig` will directly apply to the Docker engine.
