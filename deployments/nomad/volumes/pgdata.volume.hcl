# Dynamic host volume for postgres.nomad.hcl, the cluster version of the
# "pgdata" host volume that `make nomad-agent` sets up for the dev agent.
# Create it once with `make nomad-volumes`. The folder lives on whichever node
# Nomad picks, and the postgres job is then always placed on that node.
name      = "pgdata"
type      = "host"
plugin_id = "mkdir"

capability {
  access_mode     = "single-node-single-writer"
  attachment_mode = "file-system"
}

# The postgres image starts as root, then switches to the postgres user
# (uid 999) and has to get into this folder. Owned by root with the plugin's
# default 0700, it can't, and fails with "cannot create directory".
parameters {
  mode = "0700"
  uid  = 999
  gid  = 999
}
