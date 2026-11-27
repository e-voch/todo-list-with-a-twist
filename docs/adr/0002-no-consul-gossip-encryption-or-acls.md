# 2. No Consul gossip encryption or ACLs for now

- Status: Accepted
- Date: 2026-11-27
- Answers: an open question in ADR 1

## Context

ADR 1 moves Patroni and service discovery to Consul, and left open whether
to turn on gossip encryption and ACLs.

The Consul servers were first set up with gossip encryption. Getting the key
to autoscaled app nodes turned out to be the hard part:

- The cloud-init file is in the repo, so the key can't go in it.
- Filling the key in through the autoscaler put it in the node's user data,
  which the autoscaler logs every minute; fluent-bit ships those logs to
  OpenSearch.
- Having each node fetch the key from a Nomad Variable at boot worked, but
  only because Nomad ACLs are off, and it added moving parts to every node.

Meanwhile the private network is only reachable over the WireGuard VPN, and
Nomad already runs without ACLs on the same reasoning.

## Decision

Consul runs with **no gossip encryption and no ACLs**, the same trust model
as Nomad: whoever can reach the private network is trusted.

- The core servers have no `encrypt` setting and no saved keyring.
- App nodes join with plain config from `app-node.cloud-init.yaml`.
- No gossip key is stored anywhere (it was removed from the servers and from
  Nomad Variables).

## Consequences

Good:

- Nothing secret to hand to new nodes, so autoscaling stays simple.
- Nothing secret can leak through logs or user data.
- Easier to debug while learning Consul.

Bad:

- Anyone on the VPN, or any compromised container on the private network,
  can join Consul, read every service's address, register fake services
  (which Traefik would then route to), or change Patroni's leader key.
- Nomad's own gap is wider still: without ACLs anyone inside can run any job.

Revisit together with Nomad ACLs, if more people get VPN access, or if
anything untrusted runs on the private network. Turning both on later means
a rolling change on a running cluster (encryption: allow both, then require;
ACLs: default allow, hand out tokens, then default deny), and a way to give
new nodes their key and token without putting them in logs.
