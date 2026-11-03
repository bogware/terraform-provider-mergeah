# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_usage_by_registered_user" "example" {
  start_time = "2026-10-01T00:00:00Z"
  end_time   = "2026-11-01T00:00:00Z"
}

locals {
  usage_by_registered_user = jsondecode(data.mergeah_usage_by_registered_user.example.response_json)
}
