# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_registered_user" "example" {
  origin_user_id   = "support-agent"
  origin_user_name = "Support agent"
  user_type        = "SYSTEM"
}
