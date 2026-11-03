# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_tool_access_requests" "example" {
}

locals {
  tool_access_requests = jsondecode(data.mergeah_tool_access_requests.example.response_json)
}
