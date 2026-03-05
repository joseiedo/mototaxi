// stress/customers.js — Customer WebSocket load test
// Full implementation: Phase 8
// Ramps 0→10000 concurrent WebSocket connections
import { sleep } from 'k6';

export const options = {
  vus: 1,
  duration: '10s',
};

export default function () {
  sleep(1);
}
