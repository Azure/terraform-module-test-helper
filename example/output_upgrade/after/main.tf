resource "random_pet" "pet" {}
resource "random_integer" "number" {
  max = 10
  min = 0
}

output "id" {
  value = random_integer.number.result
}

output "name" {
  value = random_pet.pet.id
}

output "complete" {
  value = {
    id   = random_integer.number.result
    name = random_pet.pet.id
  }
}