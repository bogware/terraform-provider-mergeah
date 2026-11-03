# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_connector_tool_description_override" "example" {
  connector_slug = "linear"
  tool_name      = "list_issues"
  description    = "Tools for the support agent"
}
