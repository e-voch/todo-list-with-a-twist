job "redis" {
  type = "service"

  group "cache" {
    network {
      port "redis" { to = 6379 }
    }

    service {
      name     = "redis"
      port     = "redis"
      provider = "nomad"     # Nomad's built-in service discovery, so you don't need Consul yet
    }

    task "redis" {
      driver = "docker"
      config {
        image = "redis:8"
        ports = ["redis"]
        args  = ["redis-server", "--maxmemory", "64mb", "--maxmemory-policy", "allkeys-lru"]
      }
      resources {
        # Measured 200 MHz at 450 req/s and 440 at 700 req/s (2026-11-23). Reservations should match
        # real use, so Nomad's per-node totals are true.
        cpu    = 500   # MHz
        memory = 128   # MB
      }
    }
  }
}