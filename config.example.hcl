server {
  url = "http://127.0.0.1:8080"
}

agent {
  name = "local-agent"
  token = ""
  region = "local"
  environments = ["dev"]
}

runtime = "host"

poll {
  interval = "30s"
  workers = 0
}

log {
  level = "info"
}

discovery {
  static {
    enabled = true
  }

  docker {
    enabled = false
    mode = "container"
  }

  probe "http" "server-health" {
    target = "http://127.0.0.1:8080/healthz"
    group = "core"
    environment = "dev"
    enabled = true
    interval = "15s"
    timeout = "3s"
    retry_count = 0
    aggregation = "majority_down"
  }

  # Redis probe example:
  # probe "redis" "redis-local" {
  #   target = "redis://127.0.0.1:6379"
  #   group = "datastores"
  #   environment = "dev"
  #   enabled = true
  #   interval = "15s"
  #   timeout = "3s"
  #   retry_count = 0
  #   aggregation = "majority_down"
  # }
}
