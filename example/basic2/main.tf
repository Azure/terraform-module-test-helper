resource "null_resource" "test" {}

output "resource_id2" {
  value = null_resource.test.id
}