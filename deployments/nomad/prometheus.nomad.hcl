job "prometheus" {
  type = "service"

  group "prometheus" {
    shutdown_delay = "5s"

    # Metrics history lives here, so it survives restarts and redeploys.
    # Create it once with: nomad volume create deployments/nomad/volumes/promdata.volume.hcl
    volume "promdata" {
      type            = "host"
      source          = "promdata"
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }

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
      tags     = ["metrics"] # Prometheus scrapes its own /metrics too
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
        args = [
          "--config.file=/local/prometheus.yml",
          "--storage.tsdb.path=/prometheus",
          "--storage.tsdb.retention.time=15d",
        ]
      }

      volume_mount {
        volume      = "promdata"
        destination = "/prometheus" # the --storage.tsdb.path above
      }

      template {
        destination = "local/prometheus.yml"
        data        = <<EOF
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: nomad-services
    nomad_sd_configs:
      - server: http://{{ env "attr.unique.network.ip-address" }}:4646
    relabel_configs:
      # Nomad lists every service; keep only the ones tagged "metrics".
      # Prometheus joins the tags as ",tag1,tag2,", hence the commas.
      - source_labels: [__meta_nomad_tags]
        regex: .*,metrics,.*
        action: keep
      # Name each target's job after its service: job="traefik", and so on.
      - source_labels: [__meta_nomad_service]
        target_label: job
      # Except the app, which keeps job="todo" so the dashboard queries still match.
      - source_labels: [__meta_nomad_service]
        regex: todo-app-metrics
        target_label: job
        replacement: todo
EOF
      }

      resources {
        # Measured 29 MHz at 450 req/s (2026-11-23). Reservations should match
        # real use, so Nomad's per-node totals are true.
        cpu    = 100
        memory = 512
      }
    }
  }
}
