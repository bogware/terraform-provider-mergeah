# Configure the provider as shown in examples/provider/provider.tf.
data "mergeah_registered_user_usage" "example" {
  registered_user_id = "ed2988b5-e421-429c-87ab-769fa3c1bdd8"
  start_time         = "2026-10-01T00:00:00Z"
  end_time           = "2026-11-01T00:00:00Z"
}

locals {
  registered_user_usage = jsondecode(data.mergeah_registered_user_usage.example.response_json)
}
