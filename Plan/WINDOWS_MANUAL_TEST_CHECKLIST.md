# Bedrock Server Manager (BSM) - Windows Manual Test Checklist & Verification Guide

This checklist provides step-by-step test procedures to verify Bedrock Server Manager on Windows hosts across all native subsystem features, process supervision, firewall integration, and service management.

---

## 1. Prerequisites & Environment Check

- [ ] **Windows OS Version**: Windows 10 (1809+), Windows 11, or Windows Server 2019/2022 (x64).
- [ ] **Administrator Privileges**: Open an elevated PowerShell terminal (`Run as Administrator`).
- [ ] **Visual C++ 2015–2022 x64 Runtime**:
  - Run:
    ```powershell
    Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\VisualStudio\14.0\VC\Runtimes\x64" -ErrorAction SilentlyContinue
    ```
  - If missing, verify `install.ps1` downloads and installs `vc_redist.x64.exe` automatically, or verify the Web UI displays the missing runtime warning card.

---

## 2. Installation & Service Lifecycle

### 2.1 Automated Installation via `install.ps1`
- [ ] Unzip `bedrock-server-manager-v2.0.0-beta.1-windows-amd64.zip` to a test folder (e.g. `C:\Temp\bsm-test`).
- [ ] Run:
  ```powershell
  Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
  .\scripts\install.ps1
  ```
- [ ] Verify:
  - Administrator elevation prompt appears if not already elevated.
  - Executable is copied to `C:\Program Files\BedrockServerManager\bedrock-server-manager.exe`.
  - Data directory is created at `C:\ProgramData\BedrockServerManager`.
  - Windows Defender exclusion is added for `C:\ProgramData\BedrockServerManager`.
  - UWP loopback exemption is registered (`CheckNetIsolation LoopbackExempt -s` lists `Microsoft.MinecraftUWP`).
  - Windows Firewall rule `Bedrock Server Manager Web UI` (TCP 8080) is created.
  - Service `BedrockServerManager` is registered with delayed automatic startup and started.
  - Web UI is accessible at `http://localhost:8080`.

### 2.2 CLI Service Management
- [ ] Check service status:
  ```powershell
  .\bedrock-server-manager.exe service status
  ```
  Expected: Service `BedrockServerManager` is `Running`.
- [ ] Stop service:
  ```powershell
  .\bedrock-server-manager.exe service stop
  ```
  Expected: Service stops cleanly. Verify with `Get-Service BedrockServerManager`.
- [ ] Start service:
  ```powershell
  .\bedrock-server-manager.exe service start
  ```
  Expected: Service resumes without errors.
- [ ] Portable mode test:
  - Stop service, then test running standalone:
    ```powershell
    .\bedrock-server-manager.exe --data-dir .\test-data --port 8090 --open
    ```
  - Verify browser automatically launches to `http://localhost:8090` and manager operates in portable mode.
  - Press `Ctrl+C` and verify clean shutdown with no orphan processes.

---

## 3. Web UI & BDS Configuration Verification

### 3.1 Mojang EULA Compliance
- [ ] Log in with `admin / admin`.
- [ ] Navigate to **User & Access Management** (`/users`).
- [ ] Scroll down to **Mojang Bedrock Dedicated Server (BDS) & Host Configuration**.
- [ ] Verify:
  - EULA status displays "Not Accepted" on a fresh database.
  - Clicking **"Agree & Accept EULA"** records acceptance date and admin username.
  - Badge switches to green "Accepted".
- [ ] Deploy Modal check:
  - Click **Deploy Bedrock Instance**.
  - In Wizard Step 1, verify the **Execution Engine** selector is present.
  - If EULA was not accepted yet, verify the inline EULA checkbox is displayed and required before provisioning.

### 3.2 BDS Version Resolution & Cache
- [ ] On the BDS Configuration card, click **"Check BDS Releases"**:
  - Verify `Latest Stable` and `Latest Preview` versions are resolved (e.g. `1.21.x.y`).
- [ ] Verify BDS binary caching:
  - Deploy a native instance.
  - Check `C:\ProgramData\BedrockServerManager\data\bds\<version>\`:
    - Contains `bedrock_server.exe`, `.complete` marker file, and BDS library files.
  - Click **"Purge Unused Versions"** in Web UI:
    - Verify actively deployed version is **preserved**; only orphaned cache versions are removed.

---

## 4. Native BDS Execution & Process Supervision

### 4.1 Native BDS Instance Creation
- [ ] In Deploy Modal:
  - Name: `Native Test Server`
  - Slug: `native-test`
  - Engine: `Native BDS`
  - Version: `Latest Official Stable`
  - Update Policy: `Automatic`
  - UDP Port: `19132`
- [ ] Click **Deploy Instance**.
- [ ] Verify instance starts up and status changes to `RUNNING`.

### 4.2 Win32 Process Architecture & Job Objects
- [ ] Open Task Manager or Process Explorer:
  - Locate `bedrock-server-manager.exe`.
  - Locate child process `bedrock_server.exe` under Job Object.
  - Verify:
    - `bedrock_server.exe` runs with no visible command prompt console window (`CREATE_NO_WINDOW`).
    - If memory limit was configured (e.g., 2048 MB), verify Job Object enforces memory cap (`JOB_OBJECT_LIMIT_PROCESS_MEMORY`).
    - Win32 CPU & RAM utilization metrics appear accurately on Server Card and Dashboard.

### 4.3 Console I/O & Rotated Console Logs
- [ ] In Server Dashboard -> Console:
  - Verify live server startup logs appear in real-time.
  - Send command `help` or `list`.
  - Verify output response appears in console without lag.
- [ ] Verify log file on disk:
  - Inspect `C:\ProgramData\BedrockServerManager\servers\native-test\logs\`.
  - Verify `console-YYYY-MM-DD.log` is created and actively appended.

### 4.4 Hot Backup Verification (`save hold` / `save query`)
- [ ] In Server Details -> Backups tab:
  - Click **Create Backup**.
- [ ] Verify:
  - Live console shows manager executing protocol sequence:
    1. Sends `save hold`
    2. Polls `save query` until snapshot files and byte lengths are returned
    3. Copies world files up to specified byte lengths
    4. Sends `save resume`
    5. Excludes LevelDB `LOCK` file from archive
  - Backup completes successfully and `.tar.gz` archive is listed with valid size.
  - Restore test: With server stopped, restore from backup and verify world loads without corruption.

---

## 5. Crash Recovery & Auto-Rollback

- [ ] Simulate process crash:
  - In Task Manager, terminate `bedrock_server.exe` while server is marked `RUNNING`.
- [ ] Verify Supervisor behavior:
  - Manager detects termination within 1 second.
  - Backoff restart activates (restarts after 2s).
- [ ] Crash loop rollback test:
  - Kill `bedrock_server.exe` 3 times within 60 seconds (or corrupt a config file).
  - Verify:
    - Supervisor detects crash loop (3 crashes in <60s).
    - If upgraded from a previous version, manager automatically rolls back to `previous_bds_version`.
    - If no rollback exists, server transitions to `CRASHED` state with backoff to prevent CPU saturation.

---

## 6. Windows Firewall Port Gate Verification

- [ ] Check firewall rule state machine:
  - Open elevated PowerShell and run:
    ```powershell
    netsh advfirewall firewall show rule name="BSM_PORT_GATE_19132"
    ```
- [ ] **Zero-Lease State**:
  - When no players have an active Port Gate lease:
    - Verify rule exists with `Enabled: No`.
    - External UDP packets to 19132 are dropped by Windows Firewall default block policy.
- [ ] **Active Lease State**:
  - Authenticate or knock for an IP lease (e.g. `192.168.1.50`).
  - Re-run `show rule`:
    - Verify `Enabled: Yes`.
    - Verify `RemoteIP: 192.168.1.50/32`.
- [ ] **Lease Expiry State**:
  - After lease expires or is revoked:
    - Verify rule reverts to `Enabled: No`.

---

## 7. Dual Engine Coexistence & Host Boot Readiness

- [ ] Deploy two instances simultaneously:
  1. `Server-Native` running on `Native BDS` (Port 19132)
  2. `Server-Docker` running on `Docker Container` (Port 19134)
- [ ] Verify:
  - Both servers run concurrently on the same Windows host without port collision.
  - Native BDS uses `BSM_PORT_GATE_19132` via `netsh`.
  - Docker uses bridge mode with direct UDP bind, and Web UI correctly shows Port Gate disabled for Docker on Windows.
- [ ] Graceful Host Shutdown test:
  - Run `.\bedrock-server-manager.exe service stop` or trigger system reboot.
  - Verify manager executes `StopAllGracefully(15)`:
    - Both Native BDS and Docker instances receive graceful stop commands.
    - Saves are flushed to disk before processes terminate.

---

## 8. Uninstallation & Data Preservation

- [ ] Run standard uninstall:
  ```powershell
  .\scripts\install.ps1 -Uninstall
  ```
- [ ] Verify:
  - Windows service `BedrockServerManager` is stopped and removed.
  - Web UI firewall rule is removed.
  - Data directory `C:\ProgramData\BedrockServerManager` **remains intact** with all worlds, backups, and configs preserved.
- [ ] Run purge uninstall:
  ```powershell
  .\scripts\install.ps1 -Uninstall -PurgeData
  ```
- [ ] Verify data directory `C:\ProgramData\BedrockServerManager` is fully deleted.
