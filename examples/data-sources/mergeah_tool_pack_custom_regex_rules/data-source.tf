# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_tool_pack_custom_regex_rules" "example" {
  tool_pack_id = "a6d21b9d-6235-4f85-b175-0799fc4bfed1"
}

locals {
  tool_pack_custom_regex_rules = jsondecode(data.mergeah_tool_pack_custom_regex_rules.example.response_json)
}
