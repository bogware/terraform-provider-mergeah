# Configure the provider as shown in examples/provider/provider.tf.
resource "mergeah_connection_access" "example" {
  connection_id             = "ed2988b5-e421-429c-87ab-769fa3c1bdd8"
  excluded_registered_users = []
}
