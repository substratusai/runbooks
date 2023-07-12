# The L4 GPU does not support node autoprovisioning so precreating 0 size nodepool
resource "google_container_node_pool" "g2-standard-4" {
  name = "g2-standard-4"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = [var.zone]

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
    machine_type = "g2-standard-4"
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

resource "google_container_node_pool" "g2-standard-8" {
  name = "g2-standard-8"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = [var.zone]

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

resource "google_container_node_pool" "g2-standard-12" {
  name = "g2-standard-12"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = [var.zone]

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
    machine_type = "g2-standard-12"
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

resource "google_container_node_pool" "g2-standard-16" {
  name = "g2-standard-16"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = [var.zone]

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
    machine_type = "g2-standard-16"
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
  name = "g2-standard-24"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = [var.zone]

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

resource "google_container_node_pool" "g2-standard-32" {
  name = "g2-standard-32"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = [var.zone]

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
    machine_type = "g2-standard-32"
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

resource "google_container_node_pool" "g2-standard-48" {
  name = "g2-standard-48"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = [var.zone]

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
  name = "g2-standard-96"

  cluster            = google_container_cluster.main.id
  initial_node_count = 0
  node_locations     = [var.zone]

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
