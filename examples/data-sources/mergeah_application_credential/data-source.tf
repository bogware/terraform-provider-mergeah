# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_application_credential" "example" {
  resource_id = "134e0111-0f67-44f6-98f0-597000290bb3"
}

locals {
  application_credential = jsondecode(data.mergeah_application_credential.example.response_json)
}
