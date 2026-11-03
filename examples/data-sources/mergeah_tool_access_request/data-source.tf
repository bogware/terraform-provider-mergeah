# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_tool_access_request" "example" {
  resource_id = "134e0111-0f67-44f6-98f0-597000290bb3"
}

locals {
  tool_access_request = jsondecode(data.mergeah_tool_access_request.example.response_json)
}
