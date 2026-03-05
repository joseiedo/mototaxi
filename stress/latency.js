// stress/latency.js — End-to-end latency test
// Full implementation: Phase 8
// Measures Date.now() - emitted_at p50/p95/p99, goal p95 < 500ms
import { sleep } from 'k6';

export const options = {
  vus: 1,
  duration: '10s',
};

export default function () {
  sleep(1);
}
