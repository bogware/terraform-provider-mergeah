# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_custom_regex_rules" "example" {
}

locals {
  custom_regex_rules = jsondecode(data.mergeah_custom_regex_rules.example.response_json)
}
