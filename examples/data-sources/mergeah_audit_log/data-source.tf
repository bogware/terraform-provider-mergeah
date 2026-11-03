# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_audit_log" "example" {
}

locals {
  audit_log = jsondecode(data.mergeah_audit_log.example.response_json)
}
