# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_application_credentials" "example" {
}

locals {
  application_credentials = jsondecode(data.mergeah_application_credentials.example.response_json)
}
