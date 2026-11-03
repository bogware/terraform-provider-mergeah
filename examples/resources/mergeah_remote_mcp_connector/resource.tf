# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_remote_mcp_connector" "example" {
  name = "Support agent"
  url  = "https://mcp.example.com/mcp"
  slug = "support_agent"
  auth = { type = "none" }
}
