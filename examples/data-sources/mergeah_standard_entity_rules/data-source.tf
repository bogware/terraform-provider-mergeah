# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_standard_entity_rules" "example" {
}

locals {
  standard_entity_rules = jsondecode(data.mergeah_standard_entity_rules.example.response_json)
}
