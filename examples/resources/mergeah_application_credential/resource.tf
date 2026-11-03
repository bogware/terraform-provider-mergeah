# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_application_credential" "example" {
  connector_slug = "linear"
  client_id      = "your-oauth-client-id"
}
