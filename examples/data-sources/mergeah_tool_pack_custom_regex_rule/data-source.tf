# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_tool_pack_custom_regex_rule" "example" {
  tool_pack_id = "a6d21b9d-6235-4f85-b175-0799fc4bfed1"
  resource_id  = "134e0111-0f67-44f6-98f0-597000290bb3"
}

locals {
  tool_pack_custom_regex_rule = jsondecode(data.mergeah_tool_pack_custom_regex_rule.example.response_json)
}
