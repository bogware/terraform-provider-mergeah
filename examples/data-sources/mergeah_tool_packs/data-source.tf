# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_tool_packs" "example" {
}

locals {
  tool_packs = jsondecode(data.mergeah_tool_packs.example.response_json)
}
