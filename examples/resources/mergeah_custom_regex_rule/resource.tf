# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_custom_regex_rule" "example" {
  name            = "Support agent"
  regex           = "CUST-[0-9]{5}"
  inbound_action  = "REDACT"
  outbound_action = "BLOCK"
}
