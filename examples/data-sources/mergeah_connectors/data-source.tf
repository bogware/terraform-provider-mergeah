# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_connectors" "example" {
}

locals {
  connectors = jsondecode(data.mergeah_connectors.example.response_json)
}
