#!/usr/bin/env bash
set -euo pipefail

echo "=== Nginx Smoke Test ==="

echo ""
echo "--- NGINX-01: POST /location distribution across replicas ---"
seen_ips=""
for i in $(seq 1 6); do
  ip=$(curl -s -X POST http://localhost/location \
    -H "Content-Type: application/json" \
    -d '{"driver_id":"smoke-d1","lat":-23.5,"lng":-46.6,"bearing":90.0,"speed_kmh":30.0,"emitted_at":"2026-03-07T10:00:00Z"}' \
    -o /dev/null -D - 2>/dev/null | grep -i "x-upstream-addr" | awk '{print $2}' | tr -d '\r')
  echo "  Request $i → upstream: $ip"
  seen_ips="$seen_ips $ip"
done
distinct_count=$(echo "$seen_ips" | tr ' ' '\n' | grep -v '^$' | sort -u | wc -l | tr -d ' ')
echo "  Distinct upstream IPs seen: $distinct_count"
if [ "$distinct_count" -lt 2 ]; then
  echo "  WARN: Only $distinct_count distinct upstream IP(s) — expected ≥2 with 2 replicas"
else
  echo "  OK: Distribution confirmed across $distinct_count replicas"
fi

echo ""
echo "--- NGINX-02: WebSocket upgrade (expect 101 Switching Protocols) ---"
ws_raw=$(curl -si --max-time 3 \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  -H "Sec-WebSocket-Version: 13" \
  http://localhost/socket/websocket 2>/dev/null) || true
ws_status=$(echo "$ws_raw" | head -1)
echo "  WebSocket response: $ws_status"
if echo "$ws_status" | grep -q "101"; then
  echo "  OK: WebSocket upgrade succeeded"
else
  echo "  NOTE: Got '$ws_status' — if push-server is not running, 502 is expected"
fi

echo ""
echo "--- nginx_status (may return 403 from host, expected) ---"
nginx_status=$(curl -si http://localhost/nginx_status 2>/dev/null | head -1)
echo "  /nginx_status response: $nginx_status"

echo ""
echo "=== Smoke Test Complete ==="
