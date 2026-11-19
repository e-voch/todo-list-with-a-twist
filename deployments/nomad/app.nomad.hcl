job "app" {
  type = "service"

  group "app" {
    count = 3 # the starting count; the autoscaler adjusts it from here
    spread {
      attribute = "${node.unique.id}"
    }

    # When the autoscaler removes an instance, Nomad deregisters it first and
    # waits this long before stopping it, so Traefik stops sending it requests.
    shutdown_delay = "5s"

    scaling {
      enabled = true
      min     = 3
      max     = 9

      policy {
        evaluation_interval = "30s" # run the loop every 30s
        cooldown            = "2m"  # after a change, wait before changing again

        # CPU used, as a % of each instance's hard limit, averaged across
        # instances. It reads the app's own process_cpu_seconds_total from
        # Prometheus, because Nomad's stats (nomad-apm) are empty on this
        # cluster. 0.15 is the limit in cores (cpu = 300 MHz below, 2000 MHz
        # per core): change both together.
        check "cpu" {
          source = "prometheus"
          query  = "100 * avg(rate(process_cpu_seconds_total{job=\"todo\"}[1m])) / 0.15"

          strategy "target-value" {
            # 80% of 300 MHz is 0.12 cores, roughly 90 req/s per instance,
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
        image      = "ghcr.io/e-voch/todo-list-with-a-twist:1.7.1"
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
        image = "ghcr.io/e-voch/todo-list-with-a-twist:1.7.1"
        ports = ["http", "metrics"]
        # Cap the app at its cpu reservation instead of letting it use spare
        # CPU on the node. Each instance's capacity is then the same on a busy
        # node as on a quiet one, and the CPU scaling check's 80% really means
        # nearly full. Go reads this limit and sizes GOMAXPROCS to match.
        cpu_hard_limit = true
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
        # MHz (1 core = 2000 MHz on these nodes). With cpu_hard_limit above this
        # is the size of each instance: the most CPU it can use (0.15 cores).
        # Small on purpose, so the autoscaler adds instances in small steps
        # and a load test shows it scaling. The CPU check measures against it.
        cpu    = 300
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
        image      = "ghcr.io/e-voch/todo-list-with-a-twist:1.7.1"
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
