job "app" {
  type = "service"

  group "app" {
    count = 3
    network {
      port "http" { 
        static = 8080
        to = 8080 
        }
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
        image      = "ghcr.io/e-voch/todo-list-with-a-twist:1.2.0"
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
        image = "ghcr.io/e-voch/todo-list-with-a-twist:1.2.0"
        ports = ["http"]
        # The app reads brokers from a flag, not an env var, so pass the
        # KAFKA_BROKERS value that the template below sets.
        args = ["-brokers", "${KAFKA_BROKERS}"]
      }

      template {
        destination = "local/env"
        env         = true
        data        = <<EOF
{{ range nomadService "postgres" }}DATABASE_URL=postgres://todo:todo@{{ .Address }}:{{ .Port }}/todo?sslmode=disable{{ end }}
{{ range nomadService "redis" }}REDIS_ADDR={{ .Address }}:{{ .Port }}{{ end }}
{{ range nomadService "kafka" }}KAFKA_BROKERS={{ .Address }}:{{ .Port }}{{ end }}
EOF
      }

      resources {
        cpu    = 200
        memory = 128
      }
    }
  }

  # A separate group, so Nomad can place and scale the worker independently of the app.
  group "worker" {
    task "worker" {
      driver = "docker"
      config {
        image      = "ghcr.io/e-voch/todo-list-with-a-twist:1.2.0"
        entrypoint = ["./worker"]
        args       = ["-brokers", "${KAFKA_BROKERS}"]
      }

      template {
        destination = "local/env"
        env         = true
        data        = <<EOF
{{ range nomadService "postgres" }}DATABASE_URL=postgres://todo:todo@{{ .Address }}:{{ .Port }}/todo?sslmode=disable{{ end }}
{{ range nomadService "redis" }}REDIS_ADDR={{ .Address }}:{{ .Port }}{{ end }}
{{ range nomadService "kafka" }}KAFKA_BROKERS={{ .Address }}:{{ .Port }}{{ end }}
EOF
      }

      resources {
        cpu    = 200
        memory = 128
      }
    }
  }
}
