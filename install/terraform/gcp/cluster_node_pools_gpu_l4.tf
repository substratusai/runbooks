# Only some zones have L4 and using multizonal nodepools is important
# due to limited amount of capacity available
# source of info: https://cloud.google.com/compute/docs/gpus/gpu-regions-zones
locals {
  l4_locations = {
    "asia-south1"     = ["asia-south1-a"]
    "asia-southeast1" = ["asia-southeast1-b"]
    "europe-west4"    = ["europe-west4-a", "europe-west4-b", "europe-west4-c"]
    "us-central1"     = ["us-central1-a", "us-central1-b"]
    "us-east1"        = ["us-east1-b", "us-east1-d"]
    "us-east4"        = ["us-east4-a"]
    "us-west1"        = ["us-west1-a", "us-west1-b"]
  }
}

resource "google_container_node_pool" "g2-standard-8" {
  name  = "g2-standard-8"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = local.l4_locations[var.region]

  autoscaling {
    min_node_count  = 0
    max_node_count  = 3
    location_policy = "ANY"
  }
  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    spot         = true
    machine_type = "g2-standard-8"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 1
    }
    guest_accelerator {
      type  = "nvidia-l4"
      count = 1
    }
    gcfs_config {
      enabled = true
    }
  }
  lifecycle {
    ignore_changes = [
      initial_node_count
    ]
  }
}

resource "google_container_node_pool" "g2-standard-24" {
  name  = "g2-standard-24"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = local.l4_locations[var.region]

  autoscaling {
    min_node_count  = 0
    max_node_count  = 3
    location_policy = "ANY"
  }
  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    spot         = true
    machine_type = "g2-standard-24"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 2
    }
    guest_accelerator {
      type  = "nvidia-l4"
      count = 2
    }
    gcfs_config {
      enabled = true
    }
  }
  lifecycle {
    ignore_changes = [
      initial_node_count
    ]
  }
}

resource "google_container_node_pool" "g2-standard-48" {
  name  = "g2-standard-48"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = local.l4_locations[var.region]

  autoscaling {
    min_node_count  = 0
    max_node_count  = 3
    location_policy = "ANY"
  }
  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    spot         = true
    machine_type = "g2-standard-48"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 4
    }
    guest_accelerator {
      type  = "nvidia-l4"
      count = 4
    }
    gcfs_config {
      enabled = true
    }
  }
  lifecycle {
    ignore_changes = [
      initial_node_count
    ]
  }
}

resource "google_container_node_pool" "g2-standard-96" {
  name  = "g2-standard-96"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = local.l4_locations[var.region]

  autoscaling {
    min_node_count  = 0
    max_node_count  = 3
    location_policy = "ANY"
  }
  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    spot         = true
    machine_type = "g2-standard-96"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 8
    }
    guest_accelerator {
      type  = "nvidia-l4"
      count = 8
    }
    gcfs_config {
      enabled = true
    }
  }
  lifecycle {
    ignore_changes = [
      initial_node_count
    ]
  }
}
