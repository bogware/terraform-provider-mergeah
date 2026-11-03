# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_registered_user" "example" {
  resource_id = "134e0111-0f67-44f6-98f0-597000290bb3"
}

locals {
  registered_user = jsondecode(data.mergeah_registered_user.example.response_json)
}
