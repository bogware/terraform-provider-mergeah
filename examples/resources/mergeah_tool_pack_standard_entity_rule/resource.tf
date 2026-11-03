# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_tool_pack_standard_entity_rule" "example" {
  tool_pack_id    = "a6d21b9d-6235-4f85-b175-0799fc4bfed1"
  entity_type     = "EMAIL_ADDRESS"
  is_active       = true
  inbound_action  = "REDACT"
  outbound_action = "BLOCK"
}
