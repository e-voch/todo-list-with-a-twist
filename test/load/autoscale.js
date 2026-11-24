// Load test for the autoscaler: steps the load up past the CPU scaling
// threshold, holding each level long enough for it to react, then drops to a
// quiet level to watch it scale back down. Takes about 30 minutes.
//
// Run with:
//   docker run --rm -i grafana/k6:1.3.0 run - < test/load/autoscale.js
// Override the targets with -e BASE_URLS=http://a:8000,http://b:8000
import http from 'k6/http';
import { check } from 'k6';

// Spread requests over both Traefiks (Private-Nomad-1 and -2), like the load
// balancer would, so one Traefik isn't the bottleneck.
const TARGETS = (__ENV.BASE_URLS || 'http://10.0.0.3:8000,http://10.0.0.4:8000').split(',');

export const options = {
  scenarios: {
    autoscale: {
      executor: 'ramping-arrival-rate',
      startRate: 50,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 600,
      stages: [
        { duration: '1m', target: 150 },  // warm up
        { duration: '4m', target: 150 },  // below threshold: expect 3 to stay 3
        { duration: '1m', target: 300 },
        { duration: '5m', target: 300 },  // above ~270: expect 3 → 4
        { duration: '1m', target: 450 },
        { duration: '5m', target: 450 },  // expect → 5 or 6; the app node boots at 5
        { duration: '1m', target: 700 },
        { duration: '5m', target: 700 },  // expect → 7 or 8, the extra copies on the app node
        { duration: '1m', target: 50 },
        { duration: '6m', target: 50 },   // quiet: expect it to step back down to 3
      ],
    },
  },
  // Report pass/fail but don't abort: we want to see the whole run,
  // including what happens before new instances arrive.
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
