job "postgres" {
  type = "service"

  group "db" {
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
