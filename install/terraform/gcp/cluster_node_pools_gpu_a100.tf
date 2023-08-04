locals {
  a100_locations = {
    "asia-northeast1" = ["asia-northeast1-a", "asia-northeast1-c"]
    "asia-northeast3" = ["asia-northeast3-a", "asia-northeast3-b"]
    "asia-southeast1" = ["asia-southeast1-b", "asia-southeast1-c"]
    "europe-west4"    = ["europe-west4-a", "europe-west4-b"]
    "me-west1"        = ["me-west1-b", "me-west1-c"]
    "us-central1"     = ["us-central1-a", "us-central1-b", "us-central1-c", "us-central1-f"]
    "us-east1"        = ["us-east1-b"]
    "us-west1"        = ["us-west1-b"]
    "us-west3"        = ["us-west3-b"]
    "us-west4"        = ["us-west4-b"]
  }
}

resource "google_container_node_pool" "a2-highgpu-1g" {
  name  = "a2-highgpu-1g"
  count = var.attach_gpu_nodepools ? 1 : 0

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = local.a100_locations[var.region]

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
    machine_type = "a2-highgpu-1g"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 1
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


resource "google_container_node_pool" "a2-highgpu-2g" {
  name  = "a2-highgpu-2g"
  count = var.attach_gpu_nodepools ? 1 : 0

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = local.a100_locations[var.region]

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
    machine_type = "a2-highgpu-2g"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 4
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

resource "google_container_node_pool" "a2-highgpu-4g" {
  name  = "a2-highgpu-4g"
  count = var.attach_gpu_nodepools ? 1 : 0

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = local.a100_locations[var.region]

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
    machine_type = "a2-highgpu-4g"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 4
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

resource "google_container_node_pool" "a2-highgpu-8g" {
  name  = "a2-highgpu-8g"
  count = var.attach_gpu_nodepools ? 1 : 0

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = local.a100_locations[var.region]

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
    machine_type = "a2-highgpu-8g"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 8
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