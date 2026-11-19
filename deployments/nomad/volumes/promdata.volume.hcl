# Dynamic host volume for prometheus.nomad.hcl, so metrics history survives
# Prometheus restarting or being redeployed. A host volume lives on one node,
# so Prometheus always runs there: on Private-Nomad-2, which carries the least
# (only the Postgres replica).
name      = "promdata"
type      = "host"
plugin_id = "mkdir"
node_id   = "a0fbce64-4cf8-8823-0963-1fcedd56b515"

capability {
  access_mode     = "single-node-single-writer"
  attachment_mode = "file-system"
}

# The prometheus image runs as "nobody" (uid 65534), which has to be able to
# write here.
parameters {
  mode = "0700"
  uid  = 65534
  gid  = 65534
}
