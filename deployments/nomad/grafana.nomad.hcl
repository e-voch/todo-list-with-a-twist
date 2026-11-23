job "grafana" {
  type = "service"

  group "grafana" {
    shutdown_delay = "5s"

    network {
      port "http" {
        static = 3000
        to     = 3000
      }
    }

    service {
      name     = "grafana"
      port     = "http"
      provider = "nomad"
      check {
        type     = "http"
        path     = "/api/health"
        interval = "10s"
        timeout  = "5s"
      }
    }

    task "grafana" {
      driver = "docker"

      config {
        image = "grafana/grafana:12.0.0"
        ports = ["http"]
      }

      env {
        GF_PATHS_PROVISIONING = "/local/provisioning"
      }

      template {
        destination = "local/provisioning/datasources/prometheus.yaml"
        data        = <<EOF
apiVersion: 1
datasources:
  - name: Prometheus
    uid: prometheus # fixed, so dashboard JSON can refer to it
    type: prometheus
    access: proxy
    isDefault: true
{{ range nomadService "prometheus" }}    url: http://{{ .Address }}:{{ .Port }}
{{ end }}
EOF
      }

      template {
        destination = "local/provisioning/dashboards/files.yaml"
        data        = <<EOF
apiVersion: 1
providers:
  - name: files
    type: file
    # Edits made in the UI can't be saved over these: export the JSON instead.
    allowUiUpdates: false
    options:
      path: /local/dashboards
EOF
      }

      template {
        destination     = "local/dashboards/todo-app.json"
        data            = file("deployments/nomad/grafana/todo-app.json")
        left_delimiter  = "[[["
        right_delimiter = "]]]"
        change_mode     = "noop" # Grafana re-reads the folder by itself
      }

      # Community dashboards from grafana.com, downloaded when the task starts.
      # Pinned to a revision, so an upstream change can't break them.
      artifact {
        source      = "https://grafana.com/api/dashboards/1860/revisions/46/download"
        destination = "local/dashboards/node-exporter-full.json"
        mode        = "file"
      }
      artifact {
        source      = "https://grafana.com/api/dashboards/17346/revisions/9/download"
        destination = "local/dashboards/traefik.json"
        mode        = "file"
      }
      artifact {
        source      = "https://grafana.com/api/dashboards/14061/revisions/1/download"
        destination = "local/dashboards/go-runtime.json"
        mode        = "file"
      }

      resources {
        # Measured 74 MHz at 450 req/s (2026-11-23). Reservations should match
        # real use, so Nomad's per-node totals are true.
        cpu    = 100
        memory = 256
      }
    }
  }
}