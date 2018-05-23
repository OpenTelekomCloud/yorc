{
      "advertise_addr": "${ip_address}",
      "client_addr": "0.0.0.0",
      "data_dir": "/var/consul",
      "server": true,
      "rejoin_after_leave": true,
      "bootstrap_expect": ${server_number},
      "retry_join": ${consul_servers},
      "ui": ${consul_ui},
      "telemetry": {
            "statsd_address": "${statsd_ip}:8125"
      }
}