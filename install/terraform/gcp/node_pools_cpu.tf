resource "google_container_node_pool" "builder_1" {
  name = "builder-1"

  cluster            = google_container_cluster.main.id
  initial_node_count = 1
  node_locations     = [var.zone]

  autoscaling {
    min_node_count = 1
    max_node_count = 5
  }

  node_config {
    machine_type = "n2d-standard-8"
    ephemeral_storage_local_ssd_config {
      local_ssd_count = 1
    }
  }

  lifecycle {
    ignore_changes = [
      initial_node_count
    ]
  }
}
