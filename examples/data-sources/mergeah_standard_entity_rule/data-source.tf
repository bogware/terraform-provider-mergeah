# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_standard_entity_rule" "example" {
  entity_type = "EMAIL_ADDRESS"
}

locals {
  standard_entity_rule = jsondecode(data.mergeah_standard_entity_rule.example.response_json)
}
