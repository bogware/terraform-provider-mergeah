terraform {
  required_providers {
    mergeah = {
      source = "bogware/mergeah"
    }
  }
}

# Set MERGEAH_API_KEY in the environment.
provider "mergeah" {
  api_url = "https://ah-api.merge.dev/api/v1"
}
