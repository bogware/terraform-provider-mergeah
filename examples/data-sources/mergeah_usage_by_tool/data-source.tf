# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_usage_by_tool" "example" {
  start_time = "2026-10-01T00:00:00Z"
  end_time   = "2026-11-01T00:00:00Z"
}

locals {
  usage_by_tool = jsondecode(data.mergeah_usage_by_tool.example.response_json)
}
