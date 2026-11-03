# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_custom_regex_rule" "example" {
  resource_id = "134e0111-0f67-44f6-98f0-597000290bb3"
}

locals {
  custom_regex_rule = jsondecode(data.mergeah_custom_regex_rule.example.response_json)
}
