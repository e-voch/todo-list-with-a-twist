job "kafka" {
  type = "service"

  group "broker" {
    network {
      port "kafka" { to = 9092 }
    }

    service {
      name     = "kafka"
      port     = "kafka"
      provider = "nomad"

      check {
        type     = "tcp"
        interval = "5s"
        timeout  = "2s"
      }
    }

    task "broker" {
      driver = "docker"
      config {
        image = "apache/kafka:4.1.0"
        ports = ["kafka"]
      }

      env {
        CLUSTER_ID                     = "4L6g3nShT-eMCtK--X86sw"
        KAFKA_NODE_ID                  = "1"
        KAFKA_PROCESS_ROLES            = "broker,controller"
        KAFKA_CONTROLLER_QUORUM_VOTERS = "1@localhost:9093"
        KAFKA_LISTENERS                = "PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093"
        # Clients must be told an address they can reach from outside this container:
        # the host IP and the host port Nomad picked, not the 9092 inside the container.
        KAFKA_ADVERTISED_LISTENERS           = "PLAINTEXT://${NOMAD_HOST_IP_kafka}:${NOMAD_HOST_PORT_kafka}"
        KAFKA_LISTENER_SECURITY_PROTOCOL_MAP = "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"
        KAFKA_CONTROLLER_LISTENER_NAMES      = "CONTROLLER"

        KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR         = "1"
        KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR = "1"
        KAFKA_TRANSACTION_STATE_LOG_MIN_ISR            = "1"
        KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS         = "0"
        KAFKA_NUM_PARTITIONS                           = "1"
      }

      resources {
        cpu    = 500
        memory = 1024
      }
    }

    # Same job as kafka-init in docker-compose. It starts once the broker task has
    # started, waits for the broker to answer, then creates the topic and exits.
    task "create-topic" {
      lifecycle {
        hook    = "poststart"
        sidecar = false
      }

      driver = "docker"
      config {
        image      = "apache/kafka:4.1.0"
        entrypoint = ["sh", "-c"]
        args = [<<EOF
until /opt/kafka/bin/kafka-topics.sh --bootstrap-server ${NOMAD_HOST_ADDR_kafka} --list; do sleep 2; done
/opt/kafka/bin/kafka-topics.sh --bootstrap-server ${NOMAD_HOST_ADDR_kafka} --create --if-not-exists --topic tasks --partitions 1 --replication-factor 1
EOF
        ]
      }

      resources {
        cpu    = 100
        memory = 256
      }
    }
  }
}
