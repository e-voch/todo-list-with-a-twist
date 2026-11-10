# A read-only copy of the primary (postgres.nomad.hcl), kept up to date by
# streaming replication. On its first start it clones the primary with
# pg_basebackup, after that it only receives changes. Run it after the primary:
# it needs the replication user and slot the primary's setup task creates.
#
# Nothing fails over automatically. If the primary is lost, promote this one
# by hand (`pg_ctl promote` inside the container, or SELECT pg_promote()) and
# point the "postgres" service at it.
job "postgres-replica" {
  type = "service"

  group "db" {
    # Its own folder: a replica never shares data files with the primary.
    # On a cluster, create the volume on a different node than "pgdata",
    # otherwise one dead machine still takes out both copies.
    volume "pgdata-replica" {
      type            = "host"
      source          = "pgdata-replica"
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }

    network {
      port "db" { to = 5432 }
    }

    service {
      name     = "postgres-replica"
      port     = "db"
      provider = "nomad"
      check {
        type     = "tcp"
        interval = "5s"
        timeout  = "2s"
      }
    }

    task "postgres" {
      driver = "docker"
      config {
        image      = "postgres:18"
        ports      = ["db"]
        shm_size   = 134217728
        entrypoint = ["bash", "/local/entrypoint.sh"]
        # The primary's port is dynamic, so set where to stream from on every
        # start rather than relying on the address pg_basebackup saved in the
        # data folder. Command line settings win over the saved ones.
        args = [
          "postgres",
          "-c", "primary_conninfo=host=${PRIMARY_HOST} port=${PRIMARY_PORT} user=replicator password=replicator application_name=replica1",
          "-c", "primary_slot_name=replica1",
        ]
      }

      volume_mount {
        volume      = "pgdata-replica"
        destination = "/var/lib/postgresql"
      }

      # When the primary moves, this re-renders and Nomad restarts the replica
      # with the new address.
      template {
        destination = "local/primary.env"
        env         = true
        data        = <<EOF
{{ range nomadService "postgres" }}PRIMARY_HOST={{ .Address }}
PRIMARY_PORT={{ .Port }}{{ end }}
EOF
      }

      # Clones the primary into an empty data folder, then hands over to the
      # image's normal entrypoint, which sees an existing database and just
      # starts it. -R writes standby.signal, which keeps the server read-only
      # and following the primary.
      template {
        destination = "local/entrypoint.sh"
        data        = <<EOF
set -euo pipefail
if [ ! -s "$PGDATA/PG_VERSION" ]; then
  if [ -z "$${PRIMARY_HOST:-}" ]; then
    echo "primary is not registered in Nomad yet" >&2
    exit 1
  fi
  mkdir -p "$PGDATA"
  chown postgres:postgres "$PGDATA"
  chmod 700 "$PGDATA"
  # The primary's setup task may still be creating the user and slot.
  for attempt in $(seq 1 30); do
    if PGPASSWORD=replicator gosu postgres pg_basebackup \
        --host="$PRIMARY_HOST" --port="$PRIMARY_PORT" --username=replicator \
        --pgdata="$PGDATA" --wal-method=stream --slot=replica1 --write-recovery-conf; then
      break
    fi
    [ "$attempt" = 30 ] && exit 1
    echo "pg_basebackup failed, retrying" >&2
    sleep 2
  done
fi
exec docker-entrypoint.sh "$@"
EOF
      }

      resources {
        cpu    = 200
        memory = 256
      }
    }
  }
}
