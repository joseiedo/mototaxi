// stress/drivers.js — Driver load test
// Full implementation: Phase 8
// Ramps 0→2000 virtual drivers, each posting to /location every 2s
// Thresholds: http_req_failed < 1%, http_req_duration p95 < 100ms
import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  vus: 1,
  duration: '10s',
};

export default function () {
  http.get('http://nginx/');
  sleep(1);
}
