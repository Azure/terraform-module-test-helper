module mod {
  source = "../../"
}

output "resource_id" {
  value = module.mod.resource_id
}