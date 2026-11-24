// Steady load test: holds one request rate for a few minutes, to measure what
// each service really uses at that load (for example CPU, to set Nomad
// reservations). Same request mix as autoscale.js. Takes about 4 minutes.
//
// Run with:
//   docker run --rm -i grafana/k6:1.3.0 run - < test/load/steady.js
// Change the rate or length with -e RATE=300 -e DURATION=5m
// Override the targets with -e BASE_URLS=http://a:8000,http://b:8000
import http from 'k6/http';
import { check } from 'k6';

// Spread requests over both Traefiks (Private-Nomad-1 and -2), like the load
// balancer would.
const TARGETS = (__ENV.BASE_URLS || 'http://10.0.0.3:8000,http://10.0.0.4:8000').split(',');
const RATE = parseInt(__ENV.RATE || '450', 10);
const DURATION = __ENV.DURATION || '3m';

export const options = {
  scenarios: {
    warmup: {
      executor: 'ramping-arrival-rate',
      startRate: 0,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 600,
      stages: [{ duration: '30s', target: RATE }],
    },
    steady: {
      executor: 'constant-arrival-rate',
      startTime: '30s', // after the warmup
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 100,
      maxVUs: 600,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const base = TARGETS[Math.floor(Math.random() * TARGETS.length)];
  const roll = Math.random();

  if (roll < 0.79) {
    const page = Math.floor(Math.random() * 5) + 1;
    const res = http.get(`${base}/?page=${page}`);
    check(res, { 'browse ok': (r) => r.status === 200 });
  } else if (roll < 0.99) {
    const res = http.get(`${base}/?q=task`);
    check(res, { 'search ok': (r) => r.status === 200 });
  } else {
    // 1% creates, so the table doesn't grow much during the run.
    const res = http.post(`${base}/`, {
      ftitle: `load test ${Date.now()}`,
      fdescription: 'k6',
    }, { redirects: 0 });
    check(res, { 'create ok': (r) => r.status === 303 });
  }
}
