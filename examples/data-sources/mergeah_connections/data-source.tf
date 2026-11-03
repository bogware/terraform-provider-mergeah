# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_connections" "example" {
}

locals {
  connections = jsondecode(data.mergeah_connections.example.response_json)
}
