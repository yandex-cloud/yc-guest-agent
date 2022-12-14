output "public_instance_id" {
  value = yandex_compute_instance.yaga-vm.id
}

output "public_instance_ip" {
  value = yandex_compute_instance.yaga-vm.network_interface[0].nat_ip_address
}
