job "autoscaler" {
  type = "service"

  group "autoscaler" {
    count = 1

    network {
      port "http" {
        to = 8080
      }
    }

    service {
      name     = "autoscaler"
      port     = "http"
      provider = "nomad"
      check {
        type     = "http"
        path     = "/v1/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "autoscaler" {
      driver = "docker"

      config {
        image   = "hashicorp/nomad-autoscaler:0.6.0"
        command = "nomad-autoscaler"
        args    = ["agent", "-config", "${NOMAD_TASK_DIR}/config.hcl", "-http-bind-address", "0.0.0.0"]
        ports   = ["http"]
      }

      template {
        destination = "${NOMAD_TASK_DIR}/config.hcl"
        data        = <<EOF
nomad {
  address = "http://{{ env "attr.unique.network.ip-address" }}:4646"
}

apm "prometheus" {
  driver = "prometheus"
  config = {
    address = "{{ range nomadService "prometheus" }}http://{{ .Address }}:{{ .Port }}{{ end }}"
  }
}

strategy "target-value" {
  driver = "target-value"
}
EOF
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}