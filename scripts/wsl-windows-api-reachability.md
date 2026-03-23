# Reach the Go API from Windows (curl / Android emulator)

WSL2 uses a **separate virtual network**. A server bound to `0.0.0.0:8081` **inside Ubuntu** is reachable from **that** Linux as `127.0.0.1:8081`, but **Windows** `curl http://127.0.0.1:8081` only works if **localhost forwarding** (or a proxy) exposes that port on Windows.

The Android emulator uses **`10.0.2.2` → Windows host**; it needs the same fix as Windows `curl`.

## Option A — Mirrored networking (Windows 11, simplest if available)

1. Create or edit **`C:\Users\<You>\.wslconfig`**:

   ```ini
   [wsl2]
   networkingMode=mirrored
   ```

2. In **PowerShell (Admin)**: `wsl --shutdown`
3. Open **WSL** again, start the Go server on **8081**.
4. On **Windows**: `curl.exe http://127.0.0.1:8081/health`

If mirrored mode isn’t supported on your build, try Option B.

## Option B — One-time port forward (Admin PowerShell)

WSL’s IP can change after restarts; re-run when `curl` from Windows breaks again.

1. In **WSL**: `hostname -I` → note the **first** IPv4 (e.g. `172.x.x.x`).
2. **PowerShell as Administrator**:

   ```powershell
   $ip = "<WSL_IP_FROM_STEP_1>"
   netsh interface portproxy add v4tov4 listenaddress=127.0.0.1 listenport=8081 connectaddress=$ip connectport=8081
   ```

3. Allow through firewall if prompted, or add an inbound rule for **TCP 8081** if needed.
4. `curl.exe http://127.0.0.1:8081/health` from Windows.

To remove the rule later:

```powershell
netsh interface portproxy delete v4tov4 listenaddress=127.0.0.1 listenport=8081
```

## Option C — Run the Go server on Windows

Install **Go for Windows**, copy or clone the repo to **`C:\...`**, `cd server`, set env / `.env`, run:

```bat
go run .
```

Then **Windows** and the **emulator** (`10.0.2.2:8081`) both talk to the same process without WSL networking.

## Option D — Emulator only: `adb reverse` (does not fix Windows curl)

If only the app must reach the API and the server runs in WSL:

```bat
adb reverse tcp:8081 tcp:8081
```

Then set the app base URL to **`http://127.0.0.1:8081/`** (special case; our default is `10.0.2.2`).

---

**Summary:** Your Go server in WSL is fine; **Windows → WSL** localhost needs **mirrored mode**, **portproxy**, or **run the API on Windows**.
