// Load test: virtual users load the home page, pause, and repeat.
// Run with: make load-test URL=http://<load balancer IP>
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  // Step up the load, so a slowdown shows at which level it started.
  stages: [
    { duration: '30s', target: 50 },  // ramp up to 50 users
    { duration: '1m', target: 50 },   // hold 50
    { duration: '30s', target: 100 }, // ramp up to 100 users
    { duration: '1m', target: 100 },  // hold 100
    { duration: '15s', target: 0 },   // ramp down
  ],
  // k6 exits non-zero, and prints FAIL, if either is broken.
  thresholds: {
    http_req_duration: ['p(95)<200'], // 95% of requests under 200ms
    http_req_failed: ['rate<0.01'],   // under 1% of requests fail
  },
};

export default function () {
  const res = http.get(`${BASE_URL}/`);
  check(res, { 'status is 200': (r) => r.status === 200 });
  sleep(1); // a user pauses between page loads, so 100 users is about 100 req/s
}
