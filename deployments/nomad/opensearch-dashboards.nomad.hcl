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
        cpu    = 300
        memory = 1024
      }
    }
  }
}