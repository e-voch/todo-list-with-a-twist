# Dynamic host volume for one etcd member's data (etcd.nomad.hcl). Each core
# server needs its own "etcddata", so unlike pgdata this names the server
# instead of letting Nomad pick: the folder can never move to another node.
# One of three files (-1, -2, -3), identical apart from the server name.
# Cluster only; create once with `make nomad-volumes`.
name      = "etcddata"
type      = "host"
plugin_id = "mkdir"

constraint {
  attribute = "${node.unique.name}"
  value     = "Private-Nomad-3"
}

capability {
  access_mode     = "single-node-single-writer"
  attachment_mode = "file-system"
}
