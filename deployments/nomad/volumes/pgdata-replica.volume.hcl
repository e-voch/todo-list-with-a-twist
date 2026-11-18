name      = "pgdata-replica"
type      = "host"
plugin_id = "mkdir"
node_id = "a0fbce64-4cf8-8823-0963-1fcedd56b515"

capability {
  access_mode     = "single-node-single-writer"
  attachment_mode = "file-system"
}
parameters {
  mode = "0700"
  uid  = 999
  gid  = 999
}
