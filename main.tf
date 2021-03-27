provider "google" {
  project = var.gcloud_project
  region = var.region
  zone = var.zone
}

provider "google-beta" {
  project = var.gcloud_project
  region = var.region
  zone = var.zone
}

variable "project_services" {
  type = list(string)

  default = [
    "compute.googleapis.com",
    "iam.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "secretmanager.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "container.googleapis.com",
    "appengine.googleapis.com",
  ]
  description = "depending APIs"
}

variable "node_service_account_roles" {
  type = list(string)

  default = [
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/monitoring.viewer",
    "roles/stackdriver.resourceMetadata.writer",
  ]
  description = "depending APIs"
}

resource "google_project_service" "service" {
  count = length(var.project_services)
  project = var.gcloud_project
  service = element(var.project_services, count.index)
  disable_on_destroy = false
}

resource "google_service_account" "node" {
  account_id = "automutek8s-node"
  display_name = "service account to operate cluster node"
}

resource "google_project_iam_binding" "node" {
  depends_on = [google_project_service.service]

  count = length(var.node_service_account_roles)
  role = element(var.node_service_account_roles, count.index)
  members = [
    format("serviceAccount:%s", google_service_account.node.email)
  ] 
}

resource "google_compute_network" "primary-vpc" {
  depends_on = [google_project_service.service]
  name = format("%s-vpc", var.cluster_name)
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "primary-vpc-subnet" {
  name = format("%s-vpc-subnet", var.cluster_name)
  network = google_compute_network.primary-vpc.name
  region = var.region
  ip_cidr_range = var.primary_vpc_ip_cidr_range
  private_ip_google_access = true

  secondary_ip_range {
    ip_cidr_range = var.primary_vpc_pod_ip_cidr_range
    range_name = format("%s-pod-range", var.cluster_name)
  }

  secondary_ip_range {
    ip_cidr_range = var.primary_vpc_service_ip_cidr_range
    range_name = format("%s-service-range", var.cluster_name)
  }
}

resource "google_compute_address" "primary-vpc-nat" {
  depends_on = [google_project_service.service]

  name = format("%s-vpc-nat-ip", var.cluster_name)
  region = var.region
  address_type = "EXTERNAL"
}

resource "google_compute_address" "primary-vpc-ingress" {
  depends_on = [google_project_service.service]

  name = var.ingress_ip_resource_id
  region = var.region 
}

resource "google_compute_router" "primary-vpc-router" {
  depends_on = [google_project_service.service]

  name = format("%s-vpc-router", var.cluster_name)
  region = var.region
  network = google_compute_network.primary-vpc.self_link
}

resource "google_compute_router_nat" "primary-vpc-nat" {
  name = format("%s-vpc-nat", var.cluster_name)
  region = var.region
  router = google_compute_router.primary-vpc-router.name

  nat_ip_allocate_option = "MANUAL_ONLY"
  nat_ips = [ google_compute_address.primary-vpc-nat.self_link ]

  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"

  subnetwork {
    name                    = google_compute_subnetwork.primary-vpc-subnet.self_link
    source_ip_ranges_to_nat = [ "PRIMARY_IP_RANGE", "LIST_OF_SECONDARY_IP_RANGES"]

    secondary_ip_range_names = [
      google_compute_subnetwork.primary-vpc-subnet.secondary_ip_range.0.range_name,
      google_compute_subnetwork.primary-vpc-subnet.secondary_ip_range.1.range_name,
    ]
  }
}

resource "google_container_cluster" "primary" {
  depends_on = [google_project_service.service]

  name = var.cluster_name
  location = var.cluster_location
  network = google_compute_network.primary-vpc.self_link
  subnetwork = google_compute_subnetwork.primary-vpc-subnet.self_link

  release_channel {
    channel = "REGULAR"
  }

  private_cluster_config {
    enable_private_nodes = true
    enable_private_endpoint = false
    master_ipv4_cidr_block = var.primary_cluster_master_cidr_block
  }

  addons_config {
    cloudrun_config {
      disabled = true
    }
    horizontal_pod_autoscaling {
      disabled = true
    }
  }

  ip_allocation_policy {
    cluster_secondary_range_name  = google_compute_subnetwork.primary-vpc-subnet.secondary_ip_range.0.range_name
    services_secondary_range_name = google_compute_subnetwork.primary-vpc-subnet.secondary_ip_range.1.range_name
  }
  
  cluster_autoscaling {
    enabled = true
    resource_limits {
      maximum = 8
      minimum = 0
      resource_type = "cpu"
    }
    resource_limits {
      maximum = 4
      minimum = 0
      resource_type = "memory"
    }
  }

  node_pool {
    initial_node_count = 1
    name = "primary-pool"
    node_locations = [ var.cluster_location ]
    
    autoscaling {
      max_node_count = 4
      min_node_count = 0
    }

    management {
      auto_repair = true
      auto_upgrade = true
    }

    node_config {
      preemptible  = true
      machine_type = "e2-micro"

      metadata = {
        disable-legacy-endpoints = "true"
      }

      service_account = google_service_account.node.email
      oauth_scopes    = [
        "https://www.googleapis.com/auth/cloud-platform"
      ]

      shielded_instance_config {
        enable_integrity_monitoring = true
        enable_secure_boot = true
      }
    }
  }
}
