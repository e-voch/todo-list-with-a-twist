# Postgres with Patroni, for the postgres-ha Nomad job. Patroni runs as the
# container's main process and starts, stops and promotes Postgres itself, so
# both must live in one image.
FROM postgres:18

# The postgres image already has the PostgreSQL apt repository, which ships
# Patroni. Patroni's etcd support (etcd3 included) needs python3-etcd, which
# apt treats as one of several choices, so name it explicitly.
RUN apt-get update \
    && apt-get install -y --no-install-recommends patroni python3-etcd \
    && rm -rf /var/lib/apt/lists/*

# Postgres refuses to run as root. The data volume must be owned by this user
# (uid 999).
USER postgres

# A default config path; the Nomad job passes its own (/local/patroni.yml).
ENTRYPOINT ["patroni"]
CMD ["/etc/patroni/patroni.yml"]
