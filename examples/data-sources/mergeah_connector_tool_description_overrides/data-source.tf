# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_connector_tool_description_overrides" "example" {
}

locals {
  connector_tool_description_overrides = jsondecode(data.mergeah_connector_tool_description_overrides.example.response_json)
}
