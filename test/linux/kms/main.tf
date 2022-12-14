terraform {
  required_providers {
    yandex = {
      source = "yandex-cloud/yandex"
    }
  }
}

data "yandex_vpc_subnet" "default" {
  name = "default-ru-central1-a"
}


provider "yandex" {
  service_account_key_file = var.sa_key
  cloud_id                 = var.cloud_id
  folder_id                = var.folder_id
  zone                     = "ru-central1-a"
}

data "yandex_compute_image" "ubuntu-22-04" {
  family = "ubuntu-2204-lts"
}

resource "yandex_kms_symmetric_key" "yaga" {
  name              = "yaga-symetric-key"
  default_algorithm = "AES_128"
}

resource "yandex_kms_secret_ciphertext" "test_data" {
  key_id    = yandex_kms_symmetric_key.yaga.id
  plaintext = "much secret! very encrypted!"
}

data "template_file" "cloud_init" {
  template = file("cloud-init.tmpl.yaml")
  vars     = {
    user    = var.user
    ssh_key = var.public_ssh_key
  }
}

resource "yandex_iam_service_account" "yaga-sa" {
  name = var.instance_sa_name
}

resource "yandex_resourcemanager_folder_iam_binding" "yaga-sa-role" {
  folder_id = var.folder_id
  members   = ["serviceAccount:${yandex_iam_service_account.yaga-sa.id}"]
  role      = "kms.keys.encrypterDecrypter"
}

resource "yandex_compute_instance" "yaga-vm" {
  name               = var.instance_name
  platform_id        = "standard-v3"
  zone               = "ru-central1-a"
  service_account_id = yandex_iam_service_account.yaga-sa.id

  resources {
    cores  = "2"
    memory = "2"
  }

  boot_disk {
    initialize_params {
      image_id = data.yandex_compute_image.ubuntu-22-04.id
    }
  }

  network_interface {
    subnet_id = data.yandex_vpc_subnet.default.id
    nat       = true
  }

  metadata = {
    user-data   = data.template_file.cloud_init.rendered
    kms-secrets = jsonencode({
      "/opt/yaga/secret" = {
        keyId      = yandex_kms_symmetric_key.yaga.id
        ciphertext = yandex_kms_secret_ciphertext.test_data.ciphertext
      }
    })
  }
}
