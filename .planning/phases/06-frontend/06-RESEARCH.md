# Phase 6: Frontend - Research

**Researched:** 2026-03-07
**Domain:** Browser-native HTML/JS — Leaflet.js maps, Phoenix Channels WebSocket client, Nginx static file serving
**Confidence:** HIGH

## Summary

Phase 6 delivers two static HTML pages served by Nginx: `/track/{customer_id}` for single-driver tracking and `/overview` for fleet-wide monitoring. Both pages require no build step, no framework, and no API key.

The Phoenix Channels JavaScript client is available as a pre-built browser bundle (`phoenix.min.js`) directly from the npm package's `priv/static/` directory, making CDN loading trivial. Leaflet.js 1.9.4 provides the map layer with OpenStreetMap tiles (keyless). Smooth marker movement with bearing rotation requires two thin plugins: `leaflet-rotatedmarker` for the angle CSS transform and `leaflet.marker.slideto` for linear interpolation between GPS updates. Nginx needs two additional `location` blocks added to the existing `nginx.conf`: one to serve static HTML files from a mounted volume, and one to proxy `/location/drivers` (if ADV-01 is implemented) or handle the overview driver discovery differently.

The critical design constraint for FRONT-03 (overview) is discovering active driver IDs without a dedicated backend endpoint. The cleanest approach given the existing Redis data (`driver:{id}:latest` with 30s TTL) is to add a `GET /drivers` endpoint to the location-service (ADV-01, v2 requirement). However, since Phase 6 must work standalone, the fallback is probing `GET /location/driver-N` for N in 1..DRIVER_COUNT (known from the page URL or a hardcoded default) and rendering only those that return 200. This is viable because the simulator seeds driver IDs deterministically as `driver-1`, `driver-2`, ..., `driver-N`.

**Primary recommendation:** Serve two static HTML files from an Nginx volume mount at `/usr/share/nginx/html/`. Use `phoenix.min.js` from unpkg CDN, Leaflet 1.9.4, `leaflet-rotatedmarker` 0.2.0, and `leaflet.marker.slideto` 0.3.0 — all via CDN, no build step.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| FRONT-01 | `/track/{customer_id}` connects via Phoenix Channel, receives location updates, smoothly moves and rotates a Leaflet marker using bearing and coordinates | Phoenix.Socket + Phoenix.Channel JS API; `leaflet-rotatedmarker` for rotationAngle; `leaflet.marker.slideto` for smooth movement |
| FRONT-02 | `/track/{customer_id}` displays driver ID, current speed, last update time, and live end-to-end latency (`Date.now() - emitted_at`) per message | Channel `location_update` event payload contains `driver_id`, `speed_kmh`, `emitted_at` (RFC3339); `Date.now()` minus parsed `emitted_at` ms gives E2E latency |
| FRONT-03 | `/overview` page opens one Phoenix Channel per active driver, renders all simultaneously on one Leaflet map, color-coded by speed | Driver IDs are deterministic (`driver-1`..`driver-N`); probe `GET /location/driver-N` to find actives; open `customer:{N}` channels; OR implement ADV-01 `/drivers` endpoint |
| FRONT-04 | Frontend is a single static HTML file served by Nginx, using Leaflet.js with OpenStreetMap tiles — no framework, no API key required | Nginx `location /` block with `root` pointing to volume-mounted directory; Leaflet uses OpenStreetMap tiles at `https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png` |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Leaflet.js | 1.9.4 | Interactive map with OpenStreetMap tiles | Keyless, OSM-compatible, dominant no-framework map library |
| phoenix (JS client) | 1.8.5 | WebSocket + Phoenix Channel protocol | Official client; matches push-server Phoenix 1.8.x |
| leaflet-rotatedmarker | 0.2.0 | CSS `rotate()` transform on Leaflet markers via `rotationAngle` | Standard plugin for bearing-aware markers; no alternatives in active maintenance |
| leaflet.marker.slideto | 0.3.0 | Linear interpolation between two lat/lng coordinates using `requestAnimationFrame` | Provides smooth animation without hand-rolling interpolation math |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| OpenStreetMap tiles | N/A (tile URL) | Map background — free, no API key | Always; required by FRONT-04 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| leaflet.marker.slideto | Custom `requestAnimationFrame` interpolation | Hand-rolled is simpler to control but more code; plugin handles edge cases (e.g., stop on new position) |
| leaflet-rotatedmarker | CSS class manipulation | Plugin is the accepted standard; CSS-only requires marker icon recreation |
| CDN-delivered phoenix.min.js | Copy phoenix.js from push-server container | CDN is simpler; file copy requires Docker volume or build step |

### CDN URLs (verified — use exactly these)

```html
<!-- Leaflet CSS -->
<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" />

<!-- Leaflet JS -->
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>

<!-- Phoenix Channels client (exposes Phoenix.Socket globally) -->
<script src="https://unpkg.com/phoenix@1.8.5/priv/static/phoenix.min.js"></script>

<!-- Leaflet RotatedMarker plugin -->
<script src="https://cdn.jsdelivr.net/npm/leaflet-rotatedmarker@0.2.0/leaflet.rotatedMarker.js"></script>

<!-- Leaflet SlideTo plugin -->
<script src="https://cdn.jsdelivr.net/npm/leaflet.marker.slideto@0.3.0/Leaflet.Marker.SlideTo.js"></script>
```

## Architecture Patterns

### Recommended Project Structure
```
nginx/
├── nginx.conf           # Existing — add static serving + /drivers proxy blocks
└── html/
    ├── track.html       # /track/{customer_id} page (FRONT-01, FRONT-02)
    └── overview.html    # /overview page (FRONT-03)
```

The `nginx/html/` volume is mounted into Nginx at `/usr/share/nginx/html/`.

### Pattern 1: Phoenix Channel Connection (Single Driver)
**What:** Connect to the push-server WebSocket, join `customer:{customer_id}` channel, receive `location_update` events.
**When to use:** `track.html` — customer ID extracted from URL path.

```javascript
// Source: https://hexdocs.pm/phoenix/js/ and https://unpkg.com/phoenix@1.8.5/priv/static/phoenix.min.js
const customerId = window.location.pathname.split("/").pop();
const socket = new Phoenix.Socket("/socket");
socket.connect();

const channel = socket.channel(`customer:${customerId}`, {});
channel.on("location_update", (msg) => {
  // msg fields: driver_id, lat, lng, bearing, speed_kmh, emitted_at
  updateMap(msg);
  updateStats(msg);
});
channel.join()
  .receive("ok", () => console.log("joined"))
  .receive("error", (err) => console.error("join failed", err));
```

### Pattern 2: Smooth Animated Marker with Rotation
**What:** Move marker smoothly between GPS positions using `slideTo` and rotate it to the driver's bearing.
**When to use:** On every `location_update` event.

```javascript
// Source: leaflet-rotatedmarker plugin API + leaflet.marker.slideto API
// Initialize marker once
const marker = L.marker([lat, lng], {
  rotationAngle: 0,
  rotationOrigin: "center center"
}).addTo(map);

// On each update
function updateMap(msg) {
  marker.slideTo([msg.lat, msg.lng], {
    duration: 800,   // ms — matches ~1000ms emit interval
    keepAtCenter: false
  });
  marker.setRotationAngle(msg.bearing);
}
```

Note: `rotationAngle` takes degrees clockwise from north (same convention as the simulator's bearing field, which is 0–360 degrees).

### Pattern 3: Live End-to-End Latency Calculation
**What:** Compute latency as `Date.now() - Date.parse(emitted_at)`.
**When to use:** On every `location_update` event in `track.html`.

```javascript
// Source: FRONT-02 requirement + JS Date API
function computeLatency(emittedAt) {
  return Date.now() - Date.parse(emittedAt);
}
// emitted_at is RFC3339 (e.g. "2026-03-07T12:00:00.123Z")
// Date.parse handles RFC3339 natively in all modern browsers
```

### Pattern 4: Overview — Discovering Active Drivers
**What:** Probe known driver IDs via `GET /location/driver-N`, open one channel per live driver.
**When to use:** `overview.html` on page load — FRONT-03 requires no ADV-01 backend work.

```javascript
// Driver IDs are deterministic: driver-1, driver-2, ..., driver-N
// Probe a sensible upper bound (e.g., 50 or read from URL param ?max=N)
async function discoverActiveDrivers(maxCount) {
  const active = [];
  const probes = Array.from({ length: maxCount }, (_, i) => i + 1).map(async (n) => {
    const res = await fetch(`/location/driver-${n}`);
    if (res.ok) active.push(n);
  });
  await Promise.all(probes);
  return active;
}
```

Then for each active driver N, open channel `customer:customer-N` (since the simulator seeds `customer-N → driver-N` 1:1).

### Pattern 5: Speed-Based Color Coding (FRONT-03)
**What:** Color marker icon based on `speed_kmh` band.
**When to use:** Overview map marker creation and each update.

```javascript
// Breakpoints match São Paulo simulator range: 20–60 km/h (SIM-02)
function speedColor(speedKmh) {
  if (speedKmh < 30) return "green";
  if (speedKmh < 50) return "orange";
  return "red";
}
// Use L.divIcon with a colored circle background — no external icon image needed
function makeColorIcon(color) {
  return L.divIcon({
    className: "",
    html: `<div style="width:12px;height:12px;border-radius:50%;background:${color};border:2px solid #fff;box-shadow:0 0 3px rgba(0,0,0,.5)"></div>`,
    iconSize: [12, 12],
    iconAnchor: [6, 6]
  });
}
```

### Pattern 6: Nginx Static File Serving
**What:** Serve HTML files and proxy `/drivers` (if added later).
**When to use:** `nginx.conf` additions for Phase 6.

```nginx
# Add inside the existing server {} block
location / {
    root /usr/share/nginx/html;
    index index.html;
    # Let the browser handle /track/* and /overview routing
    try_files $uri $uri.html =404;
}
```

`track.html` is served at `/track.html`; a redirect rule maps `/track/{id}` to `track.html` while preserving the path for JS to read `window.location.pathname`.

Better: use a named catch-all that rewrites `/track/` prefix to `track.html` and `/overview` to `overview.html`:

```nginx
location /track/ {
    root /usr/share/nginx/html;
    try_files /track.html =404;
}

location = /overview {
    root /usr/share/nginx/html;
    try_files /overview.html =404;
}
```

### Anti-Patterns to Avoid
- **Calling `socket.connect()` without actually connecting:** Creating `new Phoenix.Socket(...)` does NOT open the connection — you must call `socket.connect()` explicitly.
- **Using `setLatLng` directly for smooth movement:** Calling `marker.setLatLng()` teleports the marker. Use `marker.slideTo()` from the plugin for interpolation.
- **Parsing `emitted_at` with a custom parser:** `Date.parse()` handles RFC3339 natively in all modern browsers. No third-party date library needed.
- **Opening one WebSocket per driver in overview:** `Phoenix.Socket` is a single WebSocket connection that multiplexes many channels over it. Open one `Socket` and create multiple `channel` objects — do NOT open one `Socket` per driver.
- **Hardcoding driver count in HTML:** Read `?count=N` from the URL query string or use a sensible default (e.g., 20) so the overview page adapts to `DRIVER_COUNT` env var without redeployment.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Marker interpolation between GPS points | Custom `requestAnimationFrame` lerp | `leaflet.marker.slideto` 0.3.0 | Handles animation cancellation on new position, browser frame timing |
| Marker bearing rotation | CSS `transform: rotate()` per tick | `leaflet-rotatedmarker` 0.2.0 | Plugin patches Leaflet's icon update cycle; manual CSS breaks on map zoom |
| Phoenix WebSocket framing | Raw WebSocket | `phoenix.min.js` Phoenix.Socket | Phoenix uses a custom heartbeat/topic multiplexing protocol — raw WS will not decode messages |
| Map tiles | Self-hosted tiles | OpenStreetMap via Leaflet | OSM tiles are free, keyless, globally available |

**Key insight:** The Phoenix Channel protocol is NOT raw WebSocket JSON. Messages are encoded as `[join_ref, ref, topic, event, payload]` arrays with a heartbeat mechanism. Using raw WebSocket would require reimplementing the entire protocol.

## Common Pitfalls

### Pitfall 1: Phoenix.Socket endpoint URL mismatch
**What goes wrong:** Page at `http://localhost/track/5` tries to connect to `/socket` — Nginx must proxy `/socket/websocket` to the push-server (already done in Phase 5). If the URL is wrong (e.g., `/ws` or `/websocket`) the connection silently fails.
**Why it happens:** Phoenix's WebSocket path convention is `/socket/websocket`. The Nginx rule at `/socket/websocket` is already in `nginx.conf`.
**How to avoid:** Always use `/socket` as the Socket endpoint string in JS — Phoenix appends `/websocket` automatically. Verify `curl -i -N -H "Upgrade: websocket" http://localhost/socket/websocket` returns 101.

### Pitfall 2: `emitted_at` timezone parsing
**What goes wrong:** `Date.parse("2026-03-07T12:00:00Z")` returns correct UTC ms, but if the simulator emits a non-UTC RFC3339 offset (e.g., `-03:00`), the latency calculation is off.
**Why it happens:** The simulator uses `time.Now().UTC().Format(time.RFC3339Nano)` — UTC guaranteed. However it's worth knowing.
**How to avoid:** Always compute `Date.now() - Date.parse(emitted_at)`. Since `emitted_at` is UTC RFC3339, no special handling is needed.

### Pitfall 3: leaflet-rotatedmarker vs leaflet.marker.slideto load order
**What goes wrong:** If `leaflet.marker.slideto` loads before Leaflet, `L.Marker.prototype.slideTo` is undefined. If `leaflet-rotatedmarker` loads before Leaflet, `rotationAngle` option is silently ignored.
**Why it happens:** Both plugins patch `L.Marker.prototype` — they require Leaflet to be loaded first.
**How to avoid:** Script load order in HTML must be: 1) leaflet.js, 2) leaflet-rotatedmarker.js, 3) leaflet.marker.slideto.js, 4) phoenix.min.js, 5) your page script.

### Pitfall 4: One Socket per driver in overview (performance)
**What goes wrong:** Opening 10 WebSocket connections for 10 drivers causes 10x handshake overhead and 10 separate heartbeat loops.
**Why it happens:** Misunderstanding Phoenix.Socket as a one-channel connection.
**How to avoid:** Create ONE `Phoenix.Socket("/socket")`, call `.connect()` once, then call `socket.channel("customer:customer-N")` for each driver. All channels multiplex over the single WebSocket.

### Pitfall 5: Nginx returns 404 for `/track/5`
**What goes wrong:** Nginx doesn't know about client-side routing — it looks for a file at `/track/5` which doesn't exist.
**Why it happens:** Default Nginx behavior is file-system path mapping.
**How to avoid:** Use `try_files /track.html =404` for the `/track/` location block so any URL under `/track/` serves `track.html`, and the JS extracts the customer ID from `window.location.pathname`.

### Pitfall 6: Overview channel topic mismatch
**What goes wrong:** The overview page opens `customer:driver-N` instead of `customer:customer-N`.
**Why it happens:** Confusion between driver IDs and customer IDs. The push-server's `CustomerChannel` joins on topic `customer:*` and resolves `customer:{id}:driver` key in Redis.
**How to avoid:** Use `customer:customer-N` as the channel topic. Driver N corresponds to `customer-N` by the seeder's 1:1 mapping.

## Code Examples

### Full track.html skeleton
```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Track Driver</title>
  <link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" />
  <style>
    #map { height: 100vh; margin: 0; }
    #stats { position: absolute; top: 10px; right: 10px; z-index: 1000;
             background: rgba(255,255,255,.9); padding: 10px; border-radius: 4px; }
  </style>
</head>
<body>
  <div id="map"></div>
  <div id="stats">
    <div>Driver: <span id="driver-id">—</span></div>
    <div>Speed: <span id="speed">—</span> km/h</div>
    <div>Last update: <span id="last-update">—</span></div>
    <div>E2E latency: <span id="latency">—</span> ms</div>
  </div>

  <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/leaflet-rotatedmarker@0.2.0/leaflet.rotatedMarker.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/leaflet.marker.slideto@0.3.0/Leaflet.Marker.SlideTo.js"></script>
  <script src="https://unpkg.com/phoenix@1.8.5/priv/static/phoenix.min.js"></script>
  <script>
    // Source: Phoenix JS docs (hexdocs.pm/phoenix/js/)
    const customerId = window.location.pathname.split("/").filter(Boolean).pop();

    // São Paulo center
    const map = L.map("map").setView([-23.55, -46.63], 13);
    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      attribution: "&copy; OpenStreetMap contributors"
    }).addTo(map);

    let marker = null;

    const socket = new Phoenix.Socket("/socket");
    socket.connect();

    const channel = socket.channel(`customer:${customerId}`, {});
    channel.on("location_update", (msg) => {
      const latency = Date.now() - Date.parse(msg.emitted_at);
      document.getElementById("driver-id").textContent = msg.driver_id;
      document.getElementById("speed").textContent = msg.speed_kmh.toFixed(1);
      document.getElementById("last-update").textContent = new Date().toLocaleTimeString();
      document.getElementById("latency").textContent = latency;

      if (!marker) {
        marker = L.marker([msg.lat, msg.lng], {
          rotationAngle: msg.bearing,
          rotationOrigin: "center center"
        }).addTo(map);
        map.setView([msg.lat, msg.lng], 15);
      } else {
        marker.slideTo([msg.lat, msg.lng], { duration: 800 });
        marker.setRotationAngle(msg.bearing);
      }
    });
    channel.join()
      .receive("error", (err) => console.error("join error", err));
  </script>
</body>
</html>
```

### Nginx location blocks to add to nginx.conf
```nginx
# Static HTML pages (add inside the existing server {} block)
location = /overview {
    root /usr/share/nginx/html;
    try_files /overview.html =404;
}

location /track/ {
    root /usr/share/nginx/html;
    try_files /track.html =404;
}
```

And in `docker-compose.yml`, add a volume mount to the nginx service:
```yaml
volumes:
  - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
  - ./nginx/html:/usr/share/nginx/html:ro   # ADD THIS
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Bundled phoenix.js from Phoenix project assets | `unpkg.com/phoenix@1.8.5/priv/static/phoenix.min.js` | Phoenix ~1.6 | CDN delivery without npm build step |
| Google Maps (requires API key) | Leaflet + OpenStreetMap | Ongoing | FRONT-04 requires keyless — OSM is the standard |
| Manual marker position update with `setLatLng` | `leaflet.marker.slideto` smooth animation | 2016 | Required by FRONT-01 "smoothly moves" |

**Deprecated/outdated:**
- `phoenix-channels` npm package (v1.0.0, 9 years old): This is NOT the official client. Use `phoenix` npm package only.
- `use Phoenix.ChannelTest` in Elixir tests: deprecated in Phoenix 1.8 (noted in STATE.md decisions).

## Open Questions

1. **FRONT-03: Active driver discovery without ADV-01**
   - What we know: Driver IDs are `driver-1` to `driver-N`; `GET /location/driver-N` returns 200 only for live drivers (30s TTL). Customer IDs are `customer-1` to `customer-N`.
   - What's unclear: Optimal probe count — probing 1..50 with `Promise.all` is fast (~parallel HTTP) but generates up to 50 requests on page load. A simpler approach is a query param `?count=10` defaulting to `DRIVER_COUNT`.
   - Recommendation: Use `?count=N` URL query param (default 10), probe `Promise.all`, open channels only for 200 responses. This works without any backend change.

2. **leaflet-rotatedmarker compatibility with Leaflet 1.9.4**
   - What we know: Plugin docs say "compatible with 1.*". Last release 2017.
   - What's unclear: No explicit 1.9.x CI test in the repo.
   - Recommendation: Confidence is MEDIUM. If rotation does not work, fallback is `L.divIcon` with a rotated SVG arrow — pure CSS, no plugin needed.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Browser smoke test via curl + manual browser verification |
| Config file | None (static HTML — no test runner config) |
| Quick run command | `curl -s -o /dev/null -w "%{http_code}" http://localhost/track/customer-1` |
| Full suite command | `curl -s -o /dev/null -w "%{http_code}" http://localhost/overview` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FRONT-01 | `/track/{id}` shows map, marker moves and rotates | manual-only (browser rendering) | `curl -f http://localhost/track/customer-1` returns 200 | ❌ Wave 0 |
| FRONT-02 | Stats panel shows driver ID, speed, last update, E2E latency | manual-only (DOM inspection) | `curl -f http://localhost/track/customer-1` returns 200 | ❌ Wave 0 |
| FRONT-03 | `/overview` renders all active drivers | manual-only (browser rendering) | `curl -f http://localhost/overview` returns 200 | ❌ Wave 0 |
| FRONT-04 | HTML file served by Nginx, no framework, no API key | smoke (HTTP 200 + content check) | `curl -f http://localhost/track/customer-1` returns 200; `curl -f http://localhost/overview` returns 200 | ❌ Wave 0 |

**Manual-only justification:** All four requirements involve visual browser rendering (map display, marker animation, color coding) that cannot be verified via CLI. HTTP 200 smoke tests verify Nginx serves the files; visual correctness requires browser inspection.

### Sampling Rate
- **Per task commit:** `curl -f http://localhost/track/customer-1 && curl -f http://localhost/overview`
- **Per wave merge:** Same curl smoke + open browser and confirm map loads
- **Phase gate:** Both URLs return 200, map renders, markers animate on first `location_update`

### Wave 0 Gaps
- [ ] `nginx/html/track.html` — covers FRONT-01, FRONT-02, FRONT-04
- [ ] `nginx/html/overview.html` — covers FRONT-03, FRONT-04
- [ ] Nginx `nginx.conf` location blocks for `/track/` and `/overview`
- [ ] `docker-compose.yml` volume mount `./nginx/html:/usr/share/nginx/html:ro`

## Sources

### Primary (HIGH confidence)
- `https://hexdocs.pm/phoenix/js/` — Phoenix Socket and Channel JS API (Socket, channel, on, join)
- `https://leafletjs.com/download.html` — Leaflet 1.9.4 CDN URLs confirmed
- `https://cdn.jsdelivr.net/npm/phoenix@1.8.5/priv/static/` — Verified phoenix.min.js 23.71 KB exists at this path
- Project source: `push-server/lib/push_server_web/customer_channel.ex` — confirmed event name `location_update`, payload fields `driver_id`, `lat`, `lng`, `bearing`, `speed_kmh`, `emitted_at`
- Project source: `simulator/internal/seeder/seeder.go` — confirmed driver IDs `driver-N`, customer IDs `customer-N`, 1:1 mapping
- Project source: `nginx/nginx.conf` — confirmed `/socket/websocket` proxy already exists; no static serving block present

### Secondary (MEDIUM confidence)
- `https://www.jsdelivr.com/package/npm/leaflet-rotatedmarker` — version 0.2.0, MIT license; `rotationAngle` and `rotationOrigin` options documented
- `https://www.jsdelivr.com/package/npm/leaflet.marker.slideto` — version 0.3.0; `Leaflet.Marker.SlideTo.js` file confirmed at CDN path
- Elixir Forum thread — confirmed `unpkg.com/phoenix@VERSION/priv/static/phoenix.min.js` URL pattern for browser use, `Phoenix.Socket` exposed as global

### Tertiary (LOW confidence)
- leaflet-rotatedmarker compatibility with Leaflet 1.9.x: plugin docs say "1.*" but last release was 2017; no explicit 1.9 test CI found

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all CDN URLs verified by fetching directory listings; phoenix.min.js existence confirmed
- Architecture: HIGH — based directly on existing push-server channel code and seeder logic
- Pitfalls: HIGH — derived from reading existing code (channel topic convention, socket connect pattern) and verified Phoenix JS docs

**Research date:** 2026-03-07
**Valid until:** 2026-09-07 (Leaflet and Phoenix are stable; CDN URLs tied to pinned versions)
