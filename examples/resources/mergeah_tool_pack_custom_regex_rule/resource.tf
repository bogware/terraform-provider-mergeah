# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_tool_pack_custom_regex_rule" "example" {
  tool_pack_id     = "a6d21b9d-6235-4f85-b175-0799fc4bfed1"
  name             = "Support agent"
  regex            = "CUST-[0-9]{5}"
  score            = 0.5
  context_keywords = []
  is_active        = true
  inbound_action   = "REDACT"
  outbound_action  = "BLOCK"
}
