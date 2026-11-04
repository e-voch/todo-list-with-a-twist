# Extra config for the local dev agent. `make nomad-agent` replaces @DATA_DIR@
# with an absolute path (Nomad needs one) and writes the result to
# .nomad-data/agent.hcl.
#
# Host volumes are folders on this machine that jobs can mount, like the named
# volumes in docker-compose.yaml. Data in them survives the container, the job
# and the agent going away.
client {
  host_volume "pgdata" {
    path      = "@DATA_DIR@/pgdata"
    read_only = false
  }

  host_volume "kafkadata" {
    path      = "@DATA_DIR@/kafkadata"
    read_only = false
  }
}
