resource "null_resource" "test" {}

output "resource_id" {
  value = null_resource.test.id
}