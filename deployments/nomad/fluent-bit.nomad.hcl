job "fluent-bit" {
  type = "system"

  group "fluent-bit" {
    volume "nomad-alloc" {
      type      = "host"
      source    = "nomad-alloc"
      read_only = true
    }

    task "fluent-bit" {
      driver = "docker"
      user   = "root"

      config {
        image   = "fluent/fluent-bit:5.1.5"
        command = "/fluent-bit/bin/fluent-bit"
        args    = ["-c", "/local/fluent-bit.conf"]
      }

      env {
        NODE_NAME = "${node.unique.name}"
      }

      volume_mount {
        volume      = "nomad-alloc"
        destination = "/nomad-alloc"
        read_only   = true
      }

      template {
        destination = "local/parsers.conf"
        data        = <<EOF
[PARSER]
    Name   nomad_path
    Format regex
    Regex  ^/nomad-alloc/(?<alloc_id>[^/]+)/alloc/logs/(?<task>[^.]+)[.](?<stream>stdout|stderr)[.][0-9]+$
EOF
      }

      template {
        destination = "local/fluent-bit.conf"
        data        = <<EOF
[SERVICE]
    Flush        5
    Log_Level    info
    Parsers_File /local/parsers.conf

[INPUT]
    Name     tail
    Path     /nomad-alloc/*/alloc/logs/*.std*.[0-9]*
    Path_Key file_path
    Tag      nomad

[FILTER]
    Name         parser
    Match        nomad
    Key_Name     file_path
    Parser       nomad_path
    Reserve_Data On
    Preserve_Key On

[FILTER]
    Name   record_modifier
    Match  nomad
    Record node $${NODE_NAME}

[OUTPUT]
    Name               opensearch
    Match              nomad
{{ range nomadService "opensearch" }}    Host               {{ .Address }}
    Port               {{ .Port }}
{{ end }}    Logstash_Format    On
    Logstash_Prefix    nomad-logs
    Suppress_Type_Name On
EOF
      }

      resources {
        cpu    = 100
        memory = 100
      }
    }
  }
}