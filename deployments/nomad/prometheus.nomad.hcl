job "prometheus" {
  type = "service"

  group "prometheus" {
    shutdown_delay = "5s"

    network {
      port "http" {
        static = 9090
        to     = 9090
      }
    }

    service {
      name     = "prometheus"
      port     = "http"
      provider = "nomad"
      check {
        type     = "http"
        path     = "/-/healthy"
        interval = "10s"
        timeout  = "5s"
      }
    }

    task "prometheus" {
      driver = "docker"

      config {
        image = "prom/prometheus:v3.5.0"
        ports = ["http"]
        # Replaces the image's default arguments, so the storage path has to be
        # repeated. There is no volume yet: history is lost when the task restarts.
        args = [
          "--config.file=/local/prometheus.yml",
          "--storage.tsdb.path=/prometheus",
          "--storage.tsdb.retention.time=15d",
        ]
      }

      # Prometheus asks the Nomad agent on its own node for the todo-app
      # instances, so a restarted or moved instance is picked up by itself.
      template {
        destination = "local/prometheus.yml"
        data        = <<EOF
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: todo
    nomad_sd_configs:
      - server: http://{{ env "attr.unique.network.ip-address" }}:4646
    relabel_configs:
      # Nomad lists every service; keep only the app's instances.
      - source_labels: [__meta_nomad_service]
        regex: todo-app
        action: keep
EOF
      }

      resources {
        cpu    = 200
        memory = 512
      }
    }
  }
}
