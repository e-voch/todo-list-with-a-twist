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
        cpu    = 100   # MHz
        memory = 128   # MB
      }
    }
  }
}