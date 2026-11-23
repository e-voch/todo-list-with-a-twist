job "traefik" {
  node_pool = "all"
  type = "system"

  group "traefik" {
    shutdown_delay = "5s"

    network {
      port "web" {
        static = 8000
        to     = 8000
      }
      port "admin" {
        static = 8081
        to     = 8081
      }
    }

    service {
      name     = "traefik"
      port     = "admin"
      provider = "nomad"
      # --metrics.prometheus serves /metrics on this admin port.
      tags     = ["metrics"]
      check {
        type     = "http"
        path     = "/ping"
        interval = "10s"
        timeout  = "2s"
      }
    }   

    task "traefik" {
      driver = "docker"

      config {
        image = "traefik:v3.4.0"
        ports = ["web", "admin"]
        args = [
          "--entrypoints.web.address=:8000",
          "--entrypoints.traefik.address=:8081",
          "--api.dashboard=true",
          "--api.insecure=true", # dashboard without login: admin port is private
          "--ping=true",
          "--metrics.prometheus=true",
          # Ask the Nomad agent on this node where the services are.
          "--providers.nomad=true",
          "--providers.nomad.endpoint.address=http://${attr.unique.network.ip-address}:4646",
          # Only route services that opt in with a traefik.enable=true tag,
          # so Postgres, Grafana and the rest are never exposed by accident.
          "--providers.nomad.exposedByDefault=false",
        ]
      }

      resources {
        # Measured 250-430 MHz at 450 req/s (2026-11-23). Reservations should match
        # real use, so Nomad's per-node totals are true.
        cpu    = 500
        # 128 was OOM-killed under load: slow backends mean many open
        # connections held at once.
        memory = 256
      }
    }
  }
}
