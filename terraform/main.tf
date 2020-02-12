terraform {
  backend "s3" {
    bucket     = "terraform-state-storage-586877430255"
    lock_table = "terraform-state-lock-586877430255"
    region     = "us-west-2"

    // THIS MUST BE UNIQUE
    key = "atlona-event-forwarder.tfstate"
  }
}

provider "aws" {
  region = "us-west-2"
}

data "aws_ssm_parameter" "eks_cluster_endpoint" {
  name = "/eks/av-cluster-endpoint"
}

provider "kubernetes" {
  host = data.aws_ssm_parameter.eks_cluster_endpoint.value
}

// pull all env vars out of ssm
data "aws_ssm_parameter" "couch_address" {
  name = "/env/couch-address"
}

data "aws_ssm_parameter" "couch_username" {
  name = "/env/couch-username"
}

data "aws_ssm_parameter" "couch_password" {
  name = "/env/couch-password"
}

data "aws_ssm_parameter" "hub_address" {
  name = "/env/hub-address"
}

data "aws_ssm_parameter" "event_processor_host" {
  name = "/env/atlona-event-forwarder/event-processor-host"
}

module "deployment" {
  source = "github.com/byuoitav/terraform//modules/kubernetes-deployment"

  // required
  name           = "atlona-event-forwarder"
  image          = "docker.pkg.github.com/byuoitav/atlona-event-forwarder/atlona-event-forwarder"
  image_version  = "v0.1.1"
  container_port = 8080
  repo_url       = "https://github.com/byuoitav/atlona-event-forwarder"

  // optional
  image_pull_secret = "github-docker-registry"
  container_env = {
    "DB_ADDRESS"           = data.aws_ssm_parameter.couch_address.value
    "DB_USERNAME"          = data.aws_ssm_parameter.couch_username.value
    "DB_PASSWORD"          = data.aws_ssm_parameter.couch_password.value
    "EVENT_PROCESSOR_HOST" = data.aws_ssm_parameter.event_processor_host.value
    "LOG_LEVEL"            = "info"
    "STOP_REPLICATION"     = "true"
  }
}
