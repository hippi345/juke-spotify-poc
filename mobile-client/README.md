# Juke Venue (Android) — mobile patron UI

Emulates what a **venue guest** sees: **now playing**, **vote for next song**, and **playlist grid** (played / vibe-fill markers). It talks to the same Go API as the web app.

## Prerequisites

- [Android Studio](https://developer.android.com/studio) (Hedgehog or newer recommended) with Android SDK 34+
- JDK 17
- The **Go server** running and reachable from the emulator

## API base URL

Default **`BuildConfig.API_BASE_URL`** is **`http://10.0.2.2:8081/`** (override with **`JUKE_API_BASE`** in `gradle.properties`). **8081** avoids a common Windows conflict where **8080** is already **PostgreSQL / EnterpriseDB** (HTML “Server is up”) instead of this Go API.

- **Android Emulator**: `10.0.2.2` is the host machine’s loopback (where the Go server should listen on the **same port** as `PORT` / default **8080**).
- **Physical device**: use your computer’s **LAN IP**, e.g. `http://192.168.1.50:8080/`, and allow cleartext or use HTTPS (see `network_security_config.xml`).

### If Logcat shows `404 Not Found` for `/api/voting/state`

A **404** means *some* HTTP server on that host:port answered, but **not** our route (or not our binary).

1. **Confirm the Go API is what’s on that port (from Windows, not only WSL)**  
   In **PowerShell**:  
   `curl.exe http://127.0.0.1:8080/`  
   You should see JSON with **`"service":"juke-spotify-poc-api"`**.  
   Then:  
   `curl.exe http://127.0.0.1:8080/api/voting/state`  
   You should get **200** and voting JSON (or an empty session), **not** 404.

2. **If `/` is not `juke-spotify-poc-api`**  
   Something else is bound to **8080** (another app, old process). Stop it or change the Go **`PORT`** and set **`JUKE_API_BASE`** to match (e.g. `http://10.0.2.2:9090/`).

3. **If Go runs only inside WSL**  
   The emulator uses **`10.0.2.2` → Windows**. The server must be reachable on **Windows `localhost:PORT`** (WSL port forwarding), or run the Go binary on Windows, or use **`adb reverse tcp:8080 tcp:8080`** and point the app at `http://127.0.0.1:8080/` per Android docs.

4. **404 body**  
   After a rebuild, unknown paths on **our** server return JSON including **`"service":"juke-spotify-poc-api"`**. If your 404 body is **HTML** or lacks that field, you’re not hitting this Go process.

## Server

- Start the Go API on **`0.0.0.0:8080`** (default `PORT`). It already enables **CORS** for local dev.
- From the **web app**, connect Spotify and **Start session** so `/api/voting/state` returns an active session.
- The mobile app only needs **public voting endpoints** (no Spotify login on the phone).

## Open in Android Studio

1. **File → Open** → select the `mobile-client` folder (this directory).
2. Let Gradle sync. If the Gradle wrapper is missing, use **File → New → Import** or run **Gradle** from the IDE to generate `gradlew`.
3. Create an **Android Virtual Device** (Pixel 6 / API 34, for example).
4. Run the **app** configuration.

## WSL note

If the Go server runs **inside WSL** and the emulator runs on **Windows**, `10.0.2.2` targets the **Windows** host, not WSL. Either:

- Run the Go server on Windows with `PORT=8080`, or  
- Expose WSL’s port to Windows and point the app at that address (or run the emulator in a setup where the host IP matches your server).

## Endpoints used

| Method | Path |
|--------|------|
| GET | `/api/voting/state` |
| POST | `/api/voting/vote` (`{"track_id":"..."}`) |
| GET | `/api/voting/playlist-overview` |

Polling interval: **2 seconds** (same idea as the web client).
