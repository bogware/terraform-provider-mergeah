# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_tool_pack_connector" "example" {
  tool_pack_id = "a6d21b9d-6235-4f85-b175-0799fc4bfed1"
  slug         = "linear"
  auth_scope   = "INDIVIDUAL"
  tool_names   = ["list_issues"]
}
