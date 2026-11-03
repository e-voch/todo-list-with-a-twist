job "todo" {
  type = "service"

  group "app" {
    network {
      port "http" { to = 8080 }
    }

    service {
      name     = "todo-app"
      port     = "http"
      provider = "nomad"
      check {
        type     = "tcp"
        interval = "10s"
        timeout  = "2s"
      }
    }

    # Runs to completion before "app" starts, like depends_on: service_completed_successfully
    task "migrate" {
      lifecycle {
        hook    = "prestart"
        sidecar = false        # false = run once and exit, rather than staying up next to the app
      }

      driver = "docker"
      config {
        image      = "ghcr.io/e-voch/todo-list-with-a-twist:v1.1.1"
        entrypoint = ["./migrate"]
        args       = ["up"]
      }

      template {
        destination = "local/env"
        env         = true
        data        = <<EOF
{{ range nomadService "postgres" }}DATABASE_URL=postgres://todo:todo@{{ .Address }}:{{ .Port }}/todo?sslmode=disable{{ end }}
EOF
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }

    task "app" {
      driver = "docker"
      config {
        image = "ghcr.io/e-voch/todo-list-with-a-twist:v1.1.1"
        ports = ["http"]
      }

      template {
        destination = "local/env"
        env         = true
        data        = <<EOF
{{ range nomadService "postgres" }}DATABASE_URL=postgres://todo:todo@{{ .Address }}:{{ .Port }}/todo?sslmode=disable{{ end }}
{{ range nomadService "redis" }}REDIS_ADDR={{ .Address }}:{{ .Port }}{{ end }}
EOF
      }

      resources {
        cpu    = 200
        memory = 128
      }
    }
  }
}
