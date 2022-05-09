module "test" {
  source = "../before/"
}

output "complete" {
  value = module.test.complete
}