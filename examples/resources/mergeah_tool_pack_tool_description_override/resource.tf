# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_tool_pack_tool_description_override" "example" {
  tool_pack_id   = "a6d21b9d-6235-4f85-b175-0799fc4bfed1"
  connector_slug = "linear"
  tool_name      = "list_issues"
  description    = "Tools for the support agent"
}
