# Nomad agent config for the 3 permanent core servers (Private-Nomad-1/2/3).
# Each runs a Nomad server and a client. Installed by hand as
# /etc/nomad.d/nomad.hcl, next to role.hcl; Nomad reads every file in that
# folder. Autoscaled app nodes use deployments/hetzner/app-node.cloud-init.yaml
# instead.
#
# The servers' own copies hard-code each server's private IP. This file finds
# it instead (same expression as the app nodes), so one file fits all three.

datacenter = "helsinki"
data_dir   = "/opt/nomad/data"
bind_addr  = "0.0.0.0"

# Talk to the other agents over the private network only.
advertise {
  http = "{{ GetPrivateInterfaces | include \"network\" \"10.0.0.0/16\" | attr \"address\" }}"
  rpc  = "{{ GetPrivateInterfaces | include \"network\" \"10.0.0.0/16\" | attr \"address\" }}"
  serf = "{{ GetPrivateInterfaces | include \"network\" \"10.0.0.0/16\" | attr \"address\" }}"
}

server {
  enabled          = true
  bootstrap_expect = 3

  server_join {
    retry_join = ["10.0.0.3", "10.0.0.4", "10.0.0.5"]
  }
}

client {
  enabled = true

  host_volume "nomad-alloc" {
    path      = "/opt/nomad/data/alloc"
    read_only = true
  }
}

# Consul agent on this server (deployments/core/consul.hcl). Nomad registers
# itself there; jobs keep using Nomad's own discovery until they switch to
# provider = "consul".
consul {
  address = "127.0.0.1:8500"
}
