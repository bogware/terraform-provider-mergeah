# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_tool_call_logs" "example" {
}

locals {
  tool_call_logs = jsondecode(data.mergeah_tool_call_logs.example.response_json)
}
