# Dynamic host volume for one Patroni member's data (postgres-ha.nomad.hcl).
# There are three volumes but only two members: the empty one is a spare, so
# when a server dies Nomad can start its member on the spare server, where
# Patroni clones the data from the leader. Each volume names
# its server. One of three files (-1, -2, -3), identical apart from the name.
# Cluster only; create once with `make nomad-volumes`.
name      = "pgha"
type      = "host"
plugin_id = "mkdir"

constraint {
  attribute = "${node.unique.name}"
  value     = "Private-Nomad-1"
}

# The image runs as the postgres user (uid 999), and Postgres refuses to start
# if anyone else can read its data folder.
parameters {
  uid  = "999"
  gid  = "999"
  mode = "0700"
}

capability {
  access_mode     = "single-node-single-writer"
  attachment_mode = "file-system"
}
