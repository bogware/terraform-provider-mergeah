# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_standard_entity_rule" "example" {
  entity_type     = "EMAIL_ADDRESS"
  is_active       = true
  inbound_action  = "REDACT"
  outbound_action = "BLOCK"
}
