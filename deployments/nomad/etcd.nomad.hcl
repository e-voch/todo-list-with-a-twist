# etcd: a 3-member key-value store that Patroni uses to hold the Postgres
# leader lock. One member per core server (meta.role = "core"), each with its
# own etcddata volume (volumes/etcddata-*.volume.hcl). The ports are static
# because members must find each other at first start, before any service
# discovery exists, and Patroni needs a fixed address: <server IP>:2379.
job "etcd" {
  type = "service"

  group "etcd" {
    count = 3

    constraint {
      attribute = "${meta.role}"
      value     = "core"
    }

    constraint {
      operator = "distinct_hosts"
      value    = "true"
    }

    volume "etcddata" {
      type            = "host"
      source          = "etcddata"
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }

    network {
      port "client" { static = 2379 }
      port "peer" { static = 2380 }
    }

    service {
      name     = "etcd"
      port     = "client"
      provider = "nomad"
      tags     = ["metrics"] # Prometheus scrapes services with this tag

      # /health also fails when this member can't reach a majority.
      check {
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "etcd" {
      driver = "docker"

      config {
        image   = "quay.io/coreos/etcd:v3.7.4"
        command = "/usr/local/bin/etcd"
        ports   = ["client", "peer"]
        args = [
          # The server's name is unique and stable, so it names the member.
          "--name", "${node.unique.name}",
          "--data-dir", "/etcd-data",
          # Listen on every address; advertise the server's real one, which
          # is what the other members and Patroni connect to.
          "--listen-client-urls", "http://0.0.0.0:2379",
          "--advertise-client-urls", "http://${NOMAD_IP_client}:2379",
          "--listen-peer-urls", "http://0.0.0.0:2380",
          "--initial-advertise-peer-urls", "http://${NOMAD_IP_peer}:2380",
          # Only read on the very first start, when the data folder is
          # empty. Replacing a server later means `etcdctl member remove`
          # and `member add`, not editing this list.
          "--initial-cluster", "Private-Nomad-1=http://10.0.0.3:2380,Private-Nomad-2=http://10.0.0.4:2380,Private-Nomad-3=http://10.0.0.5:2380",
          "--initial-cluster-token", "todo-etcd",
          "--initial-cluster-state", "new",
        ]
      }

      volume_mount {
        volume      = "etcddata"
        destination = "/etcd-data"
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
