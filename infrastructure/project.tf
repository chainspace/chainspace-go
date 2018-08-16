variable "project_name" {}
variable "node_count" {}
variable "conf_path" {}
variable "private_key_path" {}
variable "username" {}

provider "google" {
  credentials = "${file("./account.json")}"
  project = "acoustic-atom-211511"
  region     = "europe-west2"
}

resource "random_id" "id" {
  byte_length = 4
  prefix      = "${var.project_name}-"
}

data "google_container_registry_image" "chainspace" {
    name = "chainspace:latest"
}

resource "google_compute_firewall" "default" {
  name    = "chainspace-firewall"
  // network = "${google_compute_network.chainspacetestnet.name}"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["1000-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["1000-65535"]
  }

  source_ranges = ["0.0.0.0/0"]

  target_tags = ["node"]
}

resource "google_compute_instance" "default" {
  name   = "node-${format("%d", count.index+1)}"
  // machine_type = "f1-micro"
  machine_type = "n1-standard-2"
  zone = "europe-west2-b"
  tags = ["node"]

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-9"
      type = "pd-ssd"
    }
  }

  network_interface {
    network = "default"

    access_config {
      // Ephemeral IP
    }
  }

  service_account {
    scopes = ["userinfo-email", "compute-rw", "storage-ro"]
  }

  provisioner "remote-exec" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    inline = [<<EOF
     sudo mkdir -p /etc/chainspace/conf/
     sudo touch /etc/chainspace/node_id
     sudo chmod -R 777 /etc/chainspace
     sudo chmod -R 777 /etc/chainspace/node_id
     sudo echo ${count.index+1} > /etc/chainspace/node_id
     sudo apt-get update
     sudo apt-get install -y apt-transport-https ca-certificates wget software-properties-common
     wget https://download.docker.com/linux/debian/gpg
     sudo apt-key add gpg
     echo "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable" | sudo tee -a /etc/apt/sources.list.d/docker.list
     sudo apt-get update
     sudo apt-get -y install docker-ce
     sudo gcloud docker -- pull ${data.google_container_registry_image.chainspace.image_url}
     EOF
    ]
  }

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.conf_path}"
    destination = "/etc/chainspace"
  }


  provisioner "remote-exec" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    inline = [<<EOF
     sudo docker run -d --name chainspace --volume=/etc/chainspace/conf:/conf --network=host ${data.google_container_registry_image.chainspace.image_url} run --console-log info --config-root /conf testnet `cat /etc/chainspace/node_id`
     EOF
    ]
  }

  count = "${var.node_count}"
}
