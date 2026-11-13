job "opensearch" {
  type = "service"

  group "os" {
    shutdown_delay = "5s"

    volume "osdata" {
      type            = "host"
      source          = "osdata"
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }

    network {
      port "http" {
        static = 9200
        to     = 9200
      }
    }

    service {
      name     = "opensearch"
      port     = "http"
      provider = "nomad"
      check {
        type     = "http"
        path     = "/_cluster/health"
        interval = "10s"
        timeout  = "5s"
      }
    }

    task "opensearch" {
      driver = "docker"

      config {
        image   = "opensearchproject/opensearch:3.9.0"
        ports   = ["http"]
        volumes = ["local/opensearch.yml:/usr/share/opensearch/config/opensearch.yml"]
        ulimit {
          nofile  = "65536:65536"
          memlock = "-1:-1"
        }
      }

      template {
        destination = "local/opensearch.yml"
        data        = <<EOF
cluster.name: opensearch-cluster
network.host: 0.0.0.0
discovery.type: single-node
plugins.security.disabled: true
EOF
      }

      env {
        DISABLE_SECURITY_PLUGIN     = "true"
        DISABLE_INSTALL_DEMO_CONFIG = "true"
        OPENSEARCH_JAVA_OPTS        = "-Xms1g -Xmx1g"
      }

      volume_mount {
        volume      = "osdata"
        destination = "/usr/share/opensearch/data"
      }

      resources {
        cpu    = 500
        memory = 2048
      }
    }
  }
}