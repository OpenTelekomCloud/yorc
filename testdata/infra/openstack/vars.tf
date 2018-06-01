variable "ssh_key_file" {
  default = "~/.ssh/yorc.pem"
}

variable "yorc_instances" {
  default = 1
}

variable "consul_server_instances" {
  default = 1
}

variable "region" {
  default = "RegionOne"
}

variable "prefix" {
  default = ""
}

variable "ssh_manager_user" {}

variable "external_gateway" {}
variable "public_network_name" {}

variable "keystone_user" {}
variable "keystone_password" {}
variable "keystone_tenant" {}
variable "keystone_url" {}

variable "yorc_compute_image_id" {}
variable "yorc_compute_flavor_id" {}

variable "yorc_compute_manager_availability_zone" {
  default = ""
}

variable "http_proxy" {
  default = ""
}

variable "alien_download_url" {
  default = "http://fastconnect.org/maven/service/local/artifact/maven/redirect?r=opensource&g=alien4cloud&a=alien4cloud-dist&v=2.0.0&p=tar.gz&c=dist"
}
