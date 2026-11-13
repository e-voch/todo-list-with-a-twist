name      = "osdata"
type      = "host"
plugin_id = "mkdir"
node_id   = "f9bf7748-239e-a264-bd40-be82195cc7fd"

capability {
  access_mode     = "single-node-single-writer"
  attachment_mode = "file-system"
}

parameters {
  mode = "0700"
  uid  = 1000
  gid  = 1000
}