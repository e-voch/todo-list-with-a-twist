# The primary: the only Postgres that accepts writes. The app and worker
# connect to it through the "postgres" service. postgres-replica.nomad.hcl
# streams every change from it into a read-only copy.
job "postgres" {
  type = "service"

  group "db" {
    # Claim the "pgdata" host volume from the agent config. Like pg_data in
    # docker-compose.yaml: the database survives the container being replaced.
    volume "pgdata" {
      type   = "host"
      source = "pgdata"
      # Must match the volume's capability block (volumes/pgdata.volume.hcl),
      # or the scheduler finds no compatible volume.
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }

    network {
      port "db" { to = 5432 }
    }

    service {
      name     = "postgres"
      port     = "db"
      provider = "nomad"

      # Nomad marks the service unhealthy if the port stops answering.
      check {
        type     = "tcp"
        interval = "5s"
        timeout  = "2s"
      }
    }

    task "postgres" {
      driver = "docker"
      config {
        image    = "postgres:18"
        ports    = ["db"]
        shm_size = 134217728 # 128 MB in bytes, same as shm_size in your compose file
        args = [
          "postgres",
          # Use the pg_hba.conf rendered below instead of the one in the data
          # folder, so the replication rule also applies to an existing database.
          "-c", "hba_file=/local/pg_hba.conf",
          # The replica's slot makes the primary keep WAL until the replica has
          # it. Cap that, so a replica that is gone for good can't fill the disk.
          # Past the cap the replica has to be cloned again.
          "-c", "max_slot_wal_keep_size=1GB",
        ]
      }

      volume_mount {
        volume      = "pgdata"
        destination = "/var/lib/postgresql" # same path as in docker-compose.yaml
      }

      env {
        POSTGRES_USER     = "todo"
        POSTGRES_PASSWORD = "todo"
        POSTGRES_DB       = "todo"
      }

      # The image's default rules, plus one for the replica. "all" in the
      # database column doesn't cover replication connections, they need their
      # own line.
      template {
        destination = "local/pg_hba.conf"
        data        = <<EOF
local all         all                     trust
host  all         all        127.0.0.1/32 trust
host  all         all        ::1/128      trust
host  all         all        all          scram-sha-256
host  replication replicator all          scram-sha-256
EOF
      }

      resources {
        cpu    = 200
        memory = 256
      }
    }

    # Creates the replication user and slot once Postgres is up. Runs on every
    # start and skips what already exists, so it also upgrades a database that
    # was created before there was a replica.
    task "replication-setup" {
      lifecycle {
        hook    = "poststart"
        sidecar = false # run once and exit
      }

      driver = "docker"
      config {
        image   = "postgres:18"
        command = "bash"
        args    = ["/local/setup.sh"]
      }

      env {
        PGHOST     = "${NOMAD_HOST_IP_db}"
        PGPORT     = "${NOMAD_HOST_PORT_db}"
        PGUSER     = "todo"
        PGPASSWORD = "todo"
        PGDATABASE = "todo"
      }

      template {
        destination = "local/setup.sh"
        data        = <<EOF
set -euo pipefail
until pg_isready --quiet; do sleep 1; done
psql -v ON_ERROR_STOP=1 <<'SQL'
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'replicator') THEN
    CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'replicator';
  END IF;
END
$$;
SELECT pg_create_physical_replication_slot('replica1')
WHERE NOT EXISTS (SELECT FROM pg_replication_slots WHERE slot_name = 'replica1');
SQL
EOF
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
