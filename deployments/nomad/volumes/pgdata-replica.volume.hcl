# Dynamic host volume for postgres-replica.nomad.hcl, same settings as
# pgdata.volume.hcl. Create it on a different node than pgdata, for example
# by adding a node_id line, so one machine going down doesn't take both copies.
name      = "pgdata-replica"
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
