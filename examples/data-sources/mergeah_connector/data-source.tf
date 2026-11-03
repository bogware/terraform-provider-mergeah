# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_connector" "example" {
  slug = "linear"
}

locals {
  connector = jsondecode(data.mergeah_connector.example.response_json)
}
