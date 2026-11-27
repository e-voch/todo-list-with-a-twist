# 1. Use Consul for Patroni and for service discovery

- Status: Accepted
- Date: 2026-11-27
- Replaces: the etcd cluster added in PR #44 (no earlier ADR)

## Context

We are moving Postgres to Patroni so a dead primary is replaced
automatically. Patroni needs a distributed store for its leader key: a key
that only one member can hold, and that expires when the holder stops
renewing it.

What we have today:

- **etcd ×3** on the core servers (`etcd.nomad.hcl`), used only by Patroni.
  Patroni reaches it at hard-coded IPs (`10.0.0.3/4/5:2379`).
- **Nomad's built-in service discovery** (`provider = "nomad"`) for every
  service: 16 service blocks in 14 jobs, 17 `nomadService` template lookups,
  Traefik (`--providers.nomad`) and Prometheus (`nomad_sd_configs`).
- **HAProxy sidecars** (phase 3a) that send `:5000` to the leader and `:5001`
  to replicas, using Patroni's `/primary` and `/replica` health checks. Their
  server list comes from a Nomad template.
- Nothing in production uses Patroni yet; the app still talks to the old
  `postgres` job.

Options considered:

1. **Keep etcd** for Patroni and Nomad discovery for everything else.
2. **Consul for Patroni only**, everything else stays on Nomad discovery.
3. **Consul for Patroni and for all service discovery.**

## Decision

Option 3. Consul replaces both etcd and Nomad's built-in service discovery,
and we do the move before the Patroni switchover (phase 4).

- Consul servers run on the 3 core servers, and a Consul agent runs on every
  node, including autoscaled ones. They run under systemd, outside Nomad,
  because Nomad depends on them for discovery.
- Patroni keeps its leader key in Consul and registers each member with a
  `primary` or `replica` tag.
- Every job's services move to `provider = "consul"`; templates, Traefik and
  Prometheus read from Consul.
- HAProxy stays. It still decides the leader with Patroni's health checks and
  closes connections to a server that goes down. Only the source of its
  server list changes, from a Nomad template to Consul.

Why:

- One system for the leader key, the service catalog, health checks and DNS,
  instead of etcd plus Nomad discovery.
- Consul is built to work with Nomad: automatic service registration,
  templates, and Consul Connect if we want a service mesh later.
- HAProxy can follow Consul DNS directly (`server-template`), so a moved
  Patroni member doesn't require a config rewrite and reload.
- Patroni can reach Consul through the local agent, which removes the
  hard-coded etcd IPs.
- Changing Patroni's store is cheap now, while nothing in production depends
  on it. After the switchover it would mean moving a live database cluster.

## Consequences

Good:

- One place to look up any service, by DNS name or API.
- The way is open to a service mesh (mTLS between services) and other
  HashiCorp tools.
- The etcd job and its 3 volumes go away.

Bad:

- More to run: Consul servers plus an agent on every node, each with its own
  upgrades and monitoring.
- New nodes must start and join Consul before their services can be found,
  so the autoscaled node setup (`app-node.cloud-init.yaml`) gets more complex.
- The move touches nearly every job and needs one coordinated cutover.
- All Patroni failover tests (graceful stop, kill, pause) must be redone.
- The Postgres switchover is delayed until the move is done.
- Consul is under the Business Source License since 2023: free for us to use,
  but not open source in the strict sense.

Still to decide during the work:

- Gossip encryption and ACLs: set up now, or deferred like Nomad ACLs.
- Whether HAProxy reads Consul through DNS (`server-template`) or a template.
- How containers resolve `.consul` names, if anything other than HAProxy
  needs them.
