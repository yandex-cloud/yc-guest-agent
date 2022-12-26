variable "sa_key" {
  description = "Yandex Cloud Service Account key"
  sensitive   = true
}

variable "folder_id" {
  description = "Yandex Cloud Folder ID where resources will be created"
  sensitive   = true
}

variable "cloud_id" {
  description = "Yandex Cloud ID where resources will be created"
  sensitive   = true
}

variable "instance_name" {
  type    = string
  default = "yaga"
}

variable "instance_sa_name" {
  type    = string
  default = "yaga"
}

variable "public_ssh_key" {
  description = "Public SSH key"
  sensitive   = true
}

variable "user" {
  type    = string
  default = "ubuntu"
}