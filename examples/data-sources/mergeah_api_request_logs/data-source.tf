# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_api_request_logs" "example" {
}

locals {
  api_request_logs = jsondecode(data.mergeah_api_request_logs.example.response_json)
}
