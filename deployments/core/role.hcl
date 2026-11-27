# Marks a server as permanent. Jobs that keep data on the server's disk
# (postgres-ha) only run where meta.role is "core", so they never land
# on an autoscaled node that may be deleted. Installed by hand as
# /etc/nomad.d/role.hcl on Private-Nomad-1/2/3.
client {
  meta {
    role = "core"
  }
}
