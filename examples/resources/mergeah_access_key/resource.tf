# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_access_key" "example" {
  name   = "terraform-managed"
  scopes = ["management:all"]
}
