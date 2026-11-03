# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_access_keys" "example" {
}

locals {
  access_keys = jsondecode(data.mergeah_access_keys.example.response_json)
}
