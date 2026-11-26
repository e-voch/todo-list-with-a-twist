job "app" {
  type = "service"

  node_pool = "all"

  affinity {
    attribute = "${node.pool}"
    value     = "default"
    weight    = 100
  }

  group "app" {
    count = 3 # the starting count; the autoscaler adjusts it from here

    constraint {
      operator  = "distinct_property"
      attribute = "${node.unique.id}"
      value     = "2"
    }

    shutdown_delay = "5s"

    scaling {
      enabled = true
      min     = 3
      max     = 8

      policy {
        evaluation_interval = "30s" # run the loop every 30sd
        cooldown            = "2m"  # after a change, wait before changing again

        check "cpu" {
          source = "prometheus"
          # max, not avg: Traefik gives every copy the same share, so one copy at
          # its CPU limit queues requests (seen at 10 s p95) while the average
          # still looks fine.
          # 0.3 = the copy's CPU limit in cores: 600 MHz on nodes that count
          # 2000 MHz per core. Change both together.
          query  = "100 * max(rate(process_cpu_seconds_total{job=\"todo\"}[1m])) / 0.3"

          strategy "target-value" {
            # 80% of 600 MHz is 0.24 cores, roughly 180 req/s per instance,
            # leaving 20% headroom before the hard limit throttles it.
            target = 80
          }
        }
      }
    }

    network {
      port "http" {
        to = 8080
      }
      port "metrics" {
        to = 9091
      }
    }

    service {
      name     = "todo-app"
      port     = "http"
      provider = "nomad"
      # Opt in to Traefik, which sends every path to these instances.
      tags = [
        "traefik.enable=true",
        "traefik.http.routers.todo.rule=PathPrefix(`/`)",
        "traefik.http.routers.todo.entrypoints=web",
      ]
      check {
        type     = "tcp"
        interval = "10s"
        timeout  = "2s"
      }
    }

    # A separate service, so Prometheus finds the metrics port, not the app's.
    service {
      name     = "todo-app-metrics"
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

    # Runs to completion before "app" starts, like depends_on: service_completed_successfully
    task "migrate" {
      lifecycle {
        hook    = "prestart"
        sidecar = false        # false = run once and exit, rather than staying up next to the app
      }

      driver = "docker"
      config {
        image      = "ghcr.io/e-voch/todo-list-with-a-twist:1.8.3"
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
        image = "ghcr.io/e-voch/todo-list-with-a-twist:1.8.3"
        ports = ["http", "metrics"]
        cpu_hard_limit = true
        args = ["-brokers", "${KAFKA_BROKERS}"]
      }

      template {
        destination = "local/env"
        env         = true
        data        = <<EOF
{{ range nomadService "postgres" }}DATABASE_URL=postgres://todo:todo@{{ .Address }}:{{ .Port }}/todo?sslmode=disable{{ end }}
{{ range nomadService "postgres-replica" }}DATABASE_READ_URL=postgres://todo:todo@{{ .Address }}:{{ .Port }}/todo?sslmode=disable{{ end }}
{{ range nomadService "redis" }}REDIS_ADDR={{ .Address }}:{{ .Port }}{{ end }}
{{ range nomadService "kafka" }}KAFKA_BROKERS={{ .Address }}:{{ .Port }}{{ end }}
EOF
      }

      resources {
        # Hard limit (cpu_hard_limit above). Was 300: nodes sat at ~40% CPU
        # while copies hit their limit. The query's 0.3 must match this.
        cpu    = 600
        memory = 128
      }
    }
  }

  # A separate group, so Nomad can place and scale the worker independently of the app.
  group "worker" {
    shutdown_delay = "5s"

    network {
      port "metrics" {
        to = 9091
      }
    }

    # The worker serves no web pages, only /metrics for Prometheus.
    service {
      name     = "worker-metrics"
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

    task "worker" {
      driver = "docker"
      config {
        image      = "ghcr.io/e-voch/todo-list-with-a-twist:1.8.3"
        entrypoint = ["./worker"]
        args       = ["-brokers", "${KAFKA_BROKERS}"]
        ports      = ["metrics"]
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
