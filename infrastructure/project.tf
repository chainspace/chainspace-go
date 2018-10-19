variable "project_name" {}
variable "node_count" {}
variable "conf_path" {}
variable "run_path" {}
variable "runtmux_path" {}
variable "runshardingtmux_path" {}
variable "runadapt_path" {}
variable "chainspace_path" {}
variable "private_key_path" {}
variable "username" {}
variable "zones" {
  type    = "list"
  default = ["asia-east1-b", "europe-west2-b", "northamerica-northeast1-b", "us-west2-b", "australia-southeast1-b", "europe-north1-b", "southamerica-east1-b"]
}

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

resource "google_compute_instance_template" "default" {
  name = "template"
  machine_type = "n1-standard-4"
  tags = ["node"]
  min_cpu_platform = "Intel Broadwell"

  scheduling {
    preemptible = true
    automatic_restart = false
  }

  disk {
    source_image = "debian-cloud/debian-9"
    // source_image = "cos-cloud/cos-stable"
    type = "pd-ssd"
    disk_size_gb = 100
    auto_delete = true
    boot = true
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
}

resource "google_compute_instance_from_template"  "genloadmultilong" {
  name = "node-genload-multi-long-${format("%d", count.index+1)}"
  zone = "${element(var.zones, count.index)}"
  source_instance_template = "${google_compute_instance_template.default.self_link}"

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
     sudo apt-get install -y upx htop tmux
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

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.run_path}"
    destination = "/etc/chainspace/run.sh"
  }

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.runtmux_path}"
    destination = "/etc/chainspace/runtmux.sh"
  }

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.chainspace_path}"
    destination = "/etc/chainspace/chainspace.upx"
  }

  provisioner "remote-exec" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    inline = [<<EOF
     upx -d -o /etc/chainspace/chainspace /etc/chainspace/chainspace.upx
     sudo chmod -R 777 /etc/chainspace/run.sh
     sudo chmod -R 777 /etc/chainspace/runtmux.sh
     sudo chmod -R 777 /etc/chainspace/chainspace
     EOF
    ]
  }

  count = "${var.node_count}"

}

resource "google_compute_instance_from_template"  "genloadmulti" {
  name = "node-genload-multi-${format("%d", count.index+1)}"
  zone = "${element(var.zones, count.index)}"
  source_instance_template = "${google_compute_instance_template.default.self_link}"

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
     sudo apt-get install -y upx htop
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

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.run_path}"
    destination = "/etc/chainspace/run.sh"
  }

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.chainspace_path}"
    destination = "/etc/chainspace/chainspace.upx"
  }

  provisioner "remote-exec" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    inline = [<<EOF
     upx -d -o /etc/chainspace/chainspace /etc/chainspace/chainspace.upx
     sudo chmod -R 777 /etc/chainspace/run.sh
     sudo chmod -R 777 /etc/chainspace/chainspace
     EOF
    ]
  }

  count = "${var.node_count}"

}

resource "google_compute_instance_from_template" "genloadmultiadapt" {
  name = "node-genload-multi-adapt-${format("%d", count.index+1)}"
  zone = "${element(var.zones, count.index)}"
  source_instance_template = "${google_compute_instance_template.default.self_link}"

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
     sudo apt-get install -y upx htop tmux psmisc
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

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "./runadapttmux.sh"
    destination = "/etc/chainspace/runadapt.sh"
  }

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.chainspace_path}"
    destination = "/etc/chainspace/chainspace.upx"
  }

  provisioner "remote-exec" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    inline = [<<EOF
     upx -d -o /etc/chainspace/chainspace /etc/chainspace/chainspace.upx
     sudo chmod -R 777 /etc/chainspace/runadapt.sh
     sudo chmod -R 777 /etc/chainspace/chainspace
     EOF
    ]
  }

  count = "${var.node_count}"
}

resource "google_compute_instance_from_template" "genloadadapt" {
  name = "node-genload-adapt-${format("%d", count.index+1)}"
  zone = "europe-west2-b"
  source_instance_template = "${google_compute_instance_template.default.self_link}"

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
     sudo apt-get install -y upx htop
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

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.runadapt_path}"
    destination = "/etc/chainspace/runadapt.sh"
  }

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.chainspace_path}"
    destination = "/etc/chainspace/chainspace.upx"
  }

  provisioner "remote-exec" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    inline = [<<EOF
     upx -d -o /etc/chainspace/chainspace /etc/chainspace/chainspace.upx
     sudo chmod -R 777 /etc/chainspace/runadapt.sh
     sudo chmod -R 777 /etc/chainspace/chainspace
     EOF
    ]
  }

  count = "${var.node_count}"
}

resource "google_compute_instance_from_template" "genload" {
  name = "node-genload-${format("%d", count.index+1)}"
  zone = "europe-west2-b"
  source_instance_template = "${google_compute_instance_template.default.self_link}"

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
     sudo apt-get install -y upx htop
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

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.run_path}"
    destination = "/etc/chainspace/run.sh"
  }

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.chainspace_path}"
    destination = "/etc/chainspace/chainspace.upx"
  }

  provisioner "remote-exec" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    inline = [<<EOF
     upx -d -o /etc/chainspace/chainspace /etc/chainspace/chainspace.upx
     sudo chmod -R 777 /etc/chainspace/run.sh
     sudo chmod -R 777 /etc/chainspace/chainspace
     EOF
    ]
  }

  count = "${var.node_count}"
}

resource "google_compute_instance_from_template" "sharding" {
  name = "node-sharding-${format("%d", count.index+1)}"
  zone = "europe-west2-b"
  tags = ["node"]
  source_instance_template = "${google_compute_instance_template.default.self_link}"

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
     sudo apt-get install -y upx htop tmux psmisc apt-transport-https ca-certificates curl gnupg2 software-properties-common
     curl -fsSL https://download.docker.com/linux/debian/gpg | sudo apt-key add -
     sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable"
     sudo apt-get update
     sudo apt-get install -y docker-ce
     sudo gpasswd -a $USER docker
     sudo yes | sudo gcloud auth configure-docker
     sudo docker pull gcr.io/acoustic-atom-211511/chainspace.io/contract-dummy:latest
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

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.runshardingtmux_path}"
    destination = "/etc/chainspace/runshardingtmux.sh"
  }

  provisioner "file" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    source      = "${var.chainspace_path}"
    destination = "/etc/chainspace/chainspace.upx"
  }

  provisioner "remote-exec" {
    connection {
      type = "ssh"
      user = "${var.username}"
      private_key = "${file("${var.private_key_path}")}"
    }

    inline = [<<EOF
     upx -d -o /etc/chainspace/chainspace /etc/chainspace/chainspace.upx
     sudo chmod -R 777 /etc/chainspace/runshardingtmux.sh
     sudo chmod -R 777 /etc/chainspace/chainspace
      EOF
    ]
  }
//    /etc/chainspace/runshardingtmux.sh

  count = "${var.node_count}"
}
