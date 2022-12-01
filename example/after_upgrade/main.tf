resource "null_resource" "test" {}

resource "null_resource" "failed_if_present" {
  count = 1
}

output "resource_id" {
  value = null_resource.test.id
}