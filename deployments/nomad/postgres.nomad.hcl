job "postgres" {
  type = "service"

  group "db" {
    # Claim the "pgdata" host volume from the agent config. Like pg_data in
    # docker-compose.yaml: the database survives the container being replaced.
    volume "pgdata" {
      type   = "host"
      source = "pgdata"
    }

    network {
      port "db" { to = 5432 }
    }

    service {
      name     = "postgres"
      port     = "db"
      provider = "nomad"

      # New: a health check. Nomad marks the service unhealthy if the port stops answering.
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
        shm_size = 134217728   # 128 MB in bytes, same as shm_size in your compose file
      }

      volume_mount {
        volume      = "pgdata"
        destination = "/var/lib/postgresql"   # same path as in docker-compose.yaml
      }

      env {
        POSTGRES_USER     = "todo"
        POSTGRES_PASSWORD = "todo"
        POSTGRES_DB       = "todo"
      }

      resources {
        cpu    = 200
        memory = 256
      }
    }
  }
}
