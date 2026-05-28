# sensor-network

Central backend for a small fleet of ESP32 sensor nodes. A single static Go
binary with embedded SQLite (`modernc.org/sqlite`, no CGo) and an embedded web
UI. The backend subscribes to an **external MQTT broker** (HiveMQ public by
default) — there is no broker installed on the Pi anymore.

## 1. Compile for the Pi's architecture

Go lives in WSL, so build from there. Check the Pi's arch with `uname -m`:
`armv7l` -> `arm` + `GOARM=7` (this project's Pi), `aarch64` -> `arm64`.

```bash
wsl bash -c "cd /mnt/c/dev/uni/finalproject/backend && \
  GOOS=linux GOARCH=arm GOARM=7 /usr/local/go/bin/go build -ldflags='-s -w' -o backend ."
```

For a 64-bit Pi, swap to `GOARCH=arm64` (drop `GOARM`). Produces a single
self-contained `backend` file (~10 MB, statically linked).

## 2. Copy files to the Pi (scp over SSH)

From the repo root in PowerShell (each command prompts for the Pi password).

**First-time setup** (creates the directory and copies the install script):

```powershell
ssh pi@raspberrypi.local "mkdir -p /home/pi/backend"
scp backend\backend      pi@raspberrypi.local:/home/pi/backend/backend
scp backend\install.sh   pi@raspberrypi.local:/home/pi/backend/install.sh
scp backend\.env.example pi@raspberrypi.local:/home/pi/backend/.env
```

**Subsequent deploys** (binary + env every time so config changes take effect):

```powershell
scp backend\backend      pi@raspberrypi.local:/home/pi/backend/backend
scp backend\.env.example pi@raspberrypi.local:/home/pi/backend/.env
ssh pi@raspberrypi.local "sudo systemctl restart backend"
```

Note the destination is `/home/pi/backend/.env` (no `.example`). The repo's
`.env.example` is the source of truth for non-secret values — copying it directly
as `.env` overwrites the previous one so prefix/broker changes propagate. If you
have per-Pi secrets (broker password), put them in a separate file and merge, or
edit the Pi's `.env` after this command.

## 3. Run on the Pi (over SSH)

First time (registers a systemd service that auto-starts on boot and restarts on crash):

```powershell
ssh pi@raspberrypi.local "cd /home/pi/backend && bash install.sh"
```

After copying a new binary:

```bash
sudo systemctl restart backend
```

Logs: `journalctl -u backend -f`. The backend listens on `:3000`
(`PORT`, `HOST`, `DB_PATH` overridable via `.env`).

## 4. Reach the web UI over the direct Ethernet cable

The Pi sits on `192.168.1.232` (its `br0` bridge), reachable only over the cable.
Put your laptop's Ethernet adapter on the same subnet so the browser can route to it.
Find the adapter's `ifIndex` with `Get-NetAdapter`, then in an **admin** PowerShell:

```powershell
New-NetIPAddress -InterfaceIndex 21 -IPAddress 192.168.1.10 -PrefixLength 24
```

(`21` was this machine's `Ethernet 2`; adjust if different. Pick any free
`192.168.1.x` for the laptop.) Then open **http://192.168.1.232:3000** in the browser.

> Requires IPv6 enabled on the Ethernet adapter for `raspberrypi.local` to resolve
> over the cable: `Enable-NetAdapterBinding -Name 'Ethernet 2' -ComponentID ms_tcpip6`
> (admin). Verify reachability anytime with `curl.exe http://192.168.1.232:3000/health`.

## MQTT / sensor ingest

The backend connects to `MQTT_BROKER` (default `tcp://broker.hivemq.com:1883`)
and subscribes to `MQTT_TOPIC_PREFIX/+` (default `house/+`). Each matched topic
is one node — the **last segment is the node identifier** (`room`, `central`,
`cocina`, ...). Command topics live one level deeper
(`<prefix>/<node>/central`) and are excluded by the single-level wildcard.

Expected payload on a data topic is a JSON object whose numeric fields become
readings:

```json
{"temperatura": 21.5, "humedad": 60, "sonido": 312}
```

Each numeric field is stored as one row in `readings` (metric=field name,
value=number). Non-numeric fields are ignored so the firmware can add string
fields later without breaking ingest. The node is upserted on first sight; no
registration step is required.

### ESP32 firmware contract

Each board needs a **unique** topic and client ID. With the same topic/ID on
two boards the broker disconnects the duplicate and you can't tell the nodes
apart in the DB.

```cpp
const char* topic_data = "house/room";   // unique per board (house/central, house/cocina, ...)
// ...
client.connect("Imperio_room");          // unique per board
```

### API

| Method | Path | Body / Query | Purpose |
|--------|------|--------------|---------|
| GET  | `/health` | — | uptime + status |
| GET  | `/api/devices` | — | list seen nodes |
| GET  | `/api/readings` | `?node=<name>&limit=<n>` (default 50, max 1000) | recent readings, optionally filtered by node |
| POST | `/api/command` | `{"node":"room","command":"ENCENDER"}` | publishes `command` to `<prefix>/<node>/central` |

Web UI at `/` has a "Send command" row with quick `ENCENDER` / `APAGAR` buttons
that hit `/api/command`.

### Quick test from any host with `mosquitto-clients`

```bash
# fake a data publish (becomes node 'room' in the DB)
mosquitto_pub -h broker.hivemq.com -t 'house/room' \
  -m '{"temperatura":21.5,"humedad":60,"sonido":312}'

# verify command publishing (subscribe in another terminal first)
mosquitto_sub -h broker.hivemq.com -t 'house/room/central'
curl -X POST http://192.168.1.232:3000/api/command \
  -H 'Content-Type: application/json' \
  -d '{"node":"room","command":"ENCENDER"}'
```

> **Public broker caveat.** `broker.hivemq.com` is open to the whole internet —
> anyone who guesses the topic prefix can read your sensor data or inject fake
> readings/commands. Treat it as a development convenience and move to a
> private broker (with `MQTT_USERNAME` / `MQTT_PASSWORD` in `.env`) before
> trusting any of it.
