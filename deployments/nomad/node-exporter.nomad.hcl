# Machine metrics (CPU, memory, disk, network) for every node.
job "node-exporter" {
  node_pool = "all"
  type = "system" 

  group "node-exporter" {
    shutdown_delay = "5s"

    network {
      port "metrics" {
        static = 9100
      }
    }

    service {
      name     = "node-exporter"
      port     = "metrics"
      provider = "nomad"
      tags     = ["metrics"] # Prometheus scrapes services with this tag
      check {
        type     = "http"
        path     = "/metrics"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "node-exporter" {
      driver = "docker"

      config {
        image = "prom/node-exporter:v1.9.1"
        network_mode = "host"
        args = [
          "--web.listen-address=${attr.unique.network.ip-address}:9100",
        ]
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
