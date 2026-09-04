# Load / performance test for the No Lights Go API (Grafana k6).
#
# Non-deterministic by nature: measures throughput and latency percentiles
# under concurrency. Run manually or on a schedule, NOT on every push.
#
# Usage:
#   k6 run k6/load.js -e BASE_URL=http://localhost:8000

import http from 'k6/http'
import { check, sleep } from 'k6'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8000'

export const options = {
  stages: [
    { duration: '20s', target: 10 }, // ramp up to 10 VUs
    { duration: '40s', target: 10 }, // hold
    { duration: '20s', target: 0 },  // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests under 500ms
    http_req_failed: ['rate<0.01'],   // <1% errors
  },
}

export default function () {
  const radius = http.get(`${BASE_URL}/api/consultar-radio?lon=-66.9&lat=10.5`)
  check(radius, { 'radio status 200': (r) => r.status === 200 })

  const summary = http.get(`${BASE_URL}/api/stats/summary`)
  check(summary, { 'summary status 200': (r) => r.status === 200 })

  sleep(1)
}
