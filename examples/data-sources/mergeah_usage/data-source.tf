# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_usage" "example" {
  start_time = "2026-10-01T00:00:00Z"
  end_time   = "2026-11-01T00:00:00Z"
}

locals {
  usage = jsondecode(data.mergeah_usage.example.response_json)
}
