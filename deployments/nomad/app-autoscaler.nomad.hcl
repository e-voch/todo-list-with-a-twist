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

      # The Hetzner plugin (community, github.com/AndrewChubatiuk/nomad-hcloud-autoscaler).
      # It creates and deletes app nodes through the Hetzner API.
      artifact {
        source      = "https://github.com/AndrewChubatiuk/nomad-hcloud-autoscaler/releases/download/v0.1.4/hcloud-server"
        destination = "local/plugins/hcloud-server"
        mode        = "file"
      }

      config {
        image = "hashicorp/nomad-autoscaler:0.6.0"
        entrypoint = ["/bin/sh", "-c"]
        args = [<<EOF
mkdir -p /tmp/plugins && cp /local/plugins/hcloud-server /tmp/plugins/ \
  && chmod +x /tmp/plugins/hcloud-server && exec nomad-autoscaler agent \
  -config /secrets/config.hcl -policy-dir /local/policies -http-bind-address 0.0.0.0
EOF
        ]
        ports = ["http"]
      }

      # In secrets/, because it holds the Hetzner token.
      template {
        destination = "secrets/config.hcl"
        data        = <<EOF
plugin_dir = "/tmp/plugins"

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

# The query itself says how many nodes to have.
strategy "pass-through" {
  driver = "pass-through"
}

target "hcloud-server" {
  driver = "hcloud-server"
  config = {
    # Stored with: nomad var put nomad/jobs/autoscaler hcloud_token=... ssh_key=...
    hcloud_token = "{{ with nomadVar "nomad/jobs/autoscaler" }}{{ .hcloud_token }}{{ end }}"
    # Random part of new server names: Private-Nomad-App-3f9a1c.
    hcloud_random_suffix_len = "6"
  }
}
EOF
      }

      # Cluster scaling: how many app nodes (node pool and class "app") to run.
      # The app's own scaling (copies) stays in app.nomad.hcl.
      template {
        destination = "local/policies/app-nodes.hcl"
        data        = <<EOF
scaling "app_nodes" {
  enabled = true
  min     = 0
  max     = 1

  policy {
    evaluation_interval = "1m"
    # After any change, wait before changing again. Long enough for a new node
    # to boot and take copies, so the autoscaler doesn't add a second one for
    # the same spike.
    cooldown = "5m"

    # App nodes wanted, so app slots stay under 80% full:
    #   copies / (2 per node * 0.8) = nodes needed, minus the 3 permanent ones.
    #   3-4 copies -> 0, 5-8 -> 1 (it asks for 2 at 7-8, max caps it at 1).
    # Change the 1.6 if the per-node cap in app.nomad.hcl changes, and the 3
    # if the number of permanent nodes changes.
    check "app_slots" { 
      source = "prometheus"
      query  = "clamp_min(ceil(count(up{job=\"todo\"} == 1) / 1.6) - 3, 0)"

      strategy "pass-through" {}
    }

    target "hcloud-server" {
      # Log what it would do without creating or deleting servers.
      # Set to "false" once the decisions look right.
      dry-run = "false"

      # Which Nomad nodes are app nodes (the plugin doesn't know node pools).
      node_class             = "app"
      node_drain_deadline    = "5m"
      node_purge             = "true"
      # Not least_busy: in plugin v0.4.3 it crashes on scale-in (nil pointer in
      # the SDK's least_busy.go) with Nomad 2.1.1.
      node_selector_strategy = "newest_create_index"

      # The new server, as in the console's "Add Server" form.
      hcloud_location                = "hel1"
      hcloud_image                   = "ubuntu-26.04"
      hcloud_server_type             = "cx23"
      hcloud_networks                = "12807342" # todo-prod-net
      hcloud_public_net_enable_ipv4  = "false"
      hcloud_public_net_enable_ipv6  = "false"
      hcloud_ssh_keys                = "{{ with nomadVar "nomad/jobs/autoscaler" }}{{ .ssh_key }}{{ end }}"
      # Label that marks servers the plugin owns, and the start of their names
      # (Private-Nomad-App-<6 random>). Change it only while no app node exists,
      # or the plugin loses track of the servers it made under the old one.
      hcloud_group_id                = "Private-Nomad-App"
      # lb=web: load-balancer-1 targets every server with this label, so new
      # app nodes get traffic once their Traefik passes its health check.
      hcloud_labels                  = "role=app,lb=web"

      # The cloud-init file, read from the repo when you run this job (run it
      # from the repo root). Base64 so its template braces survive this template.
      hcloud_user_data             = "${base64encode(file("deployments/hetzner/app-node.cloud-init.yaml"))}"
      hcloud_b64_user_data_encoded = "true"
    }
  }
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
