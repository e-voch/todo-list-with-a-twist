job "opensearch-dashboards" {
  type = "service"

  group "dashboards" {
    shutdown_delay = "5s"

    constraint {
      attribute = "${node.unique.name}"
      value     = "Private-Nomad-2"
    }

    network {
      port "http" {
        static = 5601
        to     = 5601
      }
    }

    service {
      name     = "opensearch-dashboards"
      port     = "http"
      provider = "nomad"
      check {
        type     = "http"
        path     = "/api/status"
        interval = "10s"
        timeout  = "5s"
      }
    }

    task "dashboards" {
      driver = "docker"

      config {
        image = "opensearchproject/opensearch-dashboards:3.9.0"
        ports = ["http"]
      }

      env {
        DISABLE_SECURITY_DASHBOARDS_PLUGIN = "true"
      }

      template {
        destination = "local/os.env"
        env         = true
        data        = <<EOF
{{ range nomadService "opensearch" }}OPENSEARCH_HOSTS=http://{{ .Address }}:{{ .Port }}{{ end }}
EOF
      }

      resources {
        # Measured 3 MHz at 450 req/s (2026-11-23). Reservations should match
        # real use, so Nomad's per-node totals are true.
        cpu    = 100
        memory = 1024
      }
    }
  }
}