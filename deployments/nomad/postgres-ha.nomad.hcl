# Postgres with automatic failover. Each member runs Patroni (image built from
# deployments/postgres-ha/Dockerfile), which starts Postgres and competes for
# the leader key in Consul (deployments/core/consul.hcl). The member holding
# the key is the
# primary; the other streams from it. If the primary dies, its key expires and
# the replica promotes itself.
#
# Two members on the core servers, three pgha volumes: the spare volume lets
# Nomad replace a member lost with its server, and Patroni clones the data
# into it from the leader.
#
# Runs beside the old postgres and postgres-replica jobs with its own empty
# volumes, so it never touches their data until the switchover.
job "postgres-ha" {
  type = "service"

  group "db" {
    count = 2

    constraint {
      attribute = "${meta.role}"
      value     = "core"
    }

    # Both copies on one server would die together.
    constraint {
      operator = "distinct_hosts"
      value    = "true"
    }

    volume "pgha" {
      type            = "host"
      source          = "pgha"
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }

    # Dynamic ports: members find each other through Consul, where each one
    # publishes its own address, not through fixed ports.
    network {
      port "db" { to = 5432 }
      port "api" { to = 8008 }
    }

    # Every member, primary or replica. HAProxy (phase 3) asks each one's
    # Patroni API whether it is the primary.
    service {
      name     = "postgres-ha"
      port     = "db"
      provider = "nomad"

      # Patroni's /health answers 200 while its Postgres is running, whatever
      # its role.
      check {
        type     = "http"
        port     = "api"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    # Patroni's REST API, which also serves Prometheus metrics on /metrics.
    service {
      name     = "postgres-ha-api"
      port     = "api"
      provider = "nomad"
      tags     = ["metrics"] # Prometheus scrapes services with this tag
    }

    task "patroni" {
      driver = "docker"

      # On a normal stop Patroni shuts Postgres down and, if it is the leader,
      # deletes its key so the other member takes over at once instead of
      # waiting for the ttl. Give it time for that before Nomad kills it.
      kill_timeout = "30s"

      config {
        image    = "ghcr.io/e-voch/postgres-ha:consul"
        ports    = ["db", "api"]
        shm_size = 134217728 # 128 MB, same as the old postgres job
        args     = ["/local/patroni.yml"]
      }

      volume_mount {
        volume      = "pgha"
        destination = "/pgha"
      }

      template {
        destination = "local/patroni.yml"
        # Restarting Patroni on a config change can cause a failover, so don't
        # let a template change restart both members at the same moment.
        splay = "30s"
        data  = <<EOF
# The cluster's name. Its keys live under service/postgres-ha/ in Consul's KV
# store, and it is also the name Patroni registers in Consul's catalog.
scope: postgres-ha
name: {{ env "node.unique.name" }} # this member's name, unique per server

consul:
  # The Consul agent on this server; 127.0.0.1 would be the container itself.
  host: {{ env "attr.unique.network.ip-address" }}:8500
  # Register this member as service postgres-ha, tagged primary or replica,
  # with a health check on its REST API.
  register_service: true

restapi:
  listen: 0.0.0.0:8008
  connect_address: {{ env "NOMAD_ADDR_api" }}

# Read only when the cluster is first created, then stored in Consul and shared
# by every member. Change them later with `patronictl edit-config`.
bootstrap:
  dcs:
    ttl: 30           # the leader key expires 30 s after its last renewal
    loop_wait: 10     # each member checks in every 10 s
    retry_timeout: 10 # how long to keep retrying Consul before giving up
    # A replica further behind than 1 MB won't promote itself.
    maximum_lag_on_failover: 1048576
    # Commits wait for the replica; non-strict, so writes go on without one.
    synchronous_mode: true
    postgresql:
      use_pg_rewind: true # an old primary rejoins as a replica without a full re-clone
      use_slots: true     # the leader keeps WAL until each replica has it
      parameters:
        # Cap that WAL, so a replica gone for good can't fill the disk.
        max_slot_wal_keep_size: 1GB
  # Runs once, on the member that creates the cluster.
  post_bootstrap: /local/post-bootstrap.sh

postgresql:
  listen: 0.0.0.0:5432
  connect_address: {{ env "NOMAD_ADDR_db" }}
  data_dir: /pgha/data
  bin_dir: /usr/lib/postgresql/18/bin
  pgpass: /tmp/pgpass
  authentication:
    superuser:
      username: postgres
      password: postgres
    replication:
      username: replicator
      password: replicator
  pg_hba:
    - local all         all                     trust
    - host  all         all        127.0.0.1/32 trust
    - host  replication replicator all          scram-sha-256
    - host  all         all        all          scram-sha-256

# No watchdog for now (decided against); Patroni would only warn without one.
watchdog:
  mode: off
EOF
      }

      # Patroni passes a connection URL for the superuser as the first
      # argument. Creates what the app expects, as the old job's env did.
      template {
        destination = "local/post-bootstrap.sh"
        perms       = "755"
        data        = <<EOF
#!/bin/bash
set -euo pipefail
psql "$1" -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE todo WITH LOGIN PASSWORD 'todo';
CREATE DATABASE todo OWNER todo;
SQL
EOF
      }

      # Small while testing with an empty database; raise to the old
      # primary's 1500 MHz / 512 MB at the switchover, when its job stops.
      resources {
        cpu    = 200
        memory = 256
      }
    }
  }
}
