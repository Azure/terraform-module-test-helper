module "destructive_change_test" {
  source = "../../"
}

output "result" {
  value = module.destructive_change_test.result
}
