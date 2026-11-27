# Consul server config for the 3 core servers (Private-Nomad-1/2/3). Installed
# by hand as /etc/consul.d/consul.hcl; the package's systemd unit reads every
# file in that folder. See docs/adr/0001-consul-for-patroni-and-service-discovery.md.
#
# No gossip encryption and no ACLs, on purpose, like Nomad's ACLs: only
# people on the WireGuard VPN can reach the private network. See
# docs/adr/0002-no-consul-gossip-encryption-or-acls.md.

datacenter = "helsinki"
data_dir   = "/opt/consul"

server           = true
bootstrap_expect = 3 # no leader is elected until all 3 servers have joined
retry_join       = ["10.0.0.3", "10.0.0.4", "10.0.0.5"]

# Gossip and server-to-server traffic on the private IP only.
bind_addr = "{{ GetPrivateInterfaces | include \"network\" \"10.0.0.0/16\" | attr \"address\" }}"

# HTTP API (8500) and DNS (8600): localhost for Nomad, plus the private IP for
# containers on this server (Patroni, HAProxy) and for laptops over WireGuard.
client_addr = "127.0.0.1 {{ GetPrivateInterfaces | include \"network\" \"10.0.0.0/16\" | attr \"address\" }}"

# Web UI at http://<server>:8500/ui
ui_config {
  enabled = true
}
