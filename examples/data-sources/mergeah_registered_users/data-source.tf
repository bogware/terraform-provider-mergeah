# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_registered_users" "example" {
}

locals {
  registered_users = jsondecode(data.mergeah_registered_users.example.response_json)
}
