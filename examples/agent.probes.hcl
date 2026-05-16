probe "http" "server-health" {
  target      = "http://127.0.0.1:8080/healthz"
  group       = "core"
  environment = "dev"
  enabled     = true
  interval    = "15s"
  timeout     = "3s"
  retry_count = 0
  aggregation = "majority_down"
}

probe "redis" "redis-cache" {
  target      = "redis://127.0.0.1:6379"
  group       = "datastores"
  environment = "dev"
  interval    = "30s"
  timeout     = "3s"
}
