# Dynamic host volume for kafka.nomad.hcl. Kafka runs as uid 1000, so the
# folder must belong to that user (locally, `make nomad-agent` uses chmod 777).
name      = "kafkadata"
type      = "host"
plugin_id = "mkdir"

capability {
  access_mode     = "single-node-single-writer"
  attachment_mode = "file-system"
}

parameters {
  mode = "0755"
  uid  = 1000
  gid  = 1000
}
