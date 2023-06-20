locals {
  node_pools = merge(
    {
      "builder-1" = {
        local_ssd_count = 1
      },
    },
    local.g2_machine_types,
  )
  g2_machine_size_count = {
    "4"  = 1,
    "8"  = 1,
    "12" = 1,
    "16" = 1,
    "24" = 2,
    # "32" = 2, # 4 doesnt work, 8 doesnt work
    "48" = 4,
    "96" = 8,
  }
  g2_machine_types = {
    # The L4 GPU does not support node autoprovisioning so precreating 0 size nodepool
    for size, cnt in local.g2_machine_size_count : "g2-standard-${size}" => {
      machine_type            = "g2-standard-${size}"
      local_ssd_count         = cnt
      guest_accelerator_count = cnt
    }
  }
  node_pool_defaults = {
    max_node_count          = 3
    initial_node_count      = 0
    machine_type            = "n1-standard-4"
    node_locations          = [var.zone]
    guest_accelerator_count = 0
  }
  _node_pools = { for np_name, config_vals in local.node_pools : np_name => merge(
    local.node_pool_defaults,
    config_vals
  ) }

}

data "google_container_engine_versions" "main" {
  provider = google-beta
  location = var.region
}

resource "google_container_cluster" "main" {
  provider = google-beta

  name    = var.name
  project = var.project_id

  location           = var.region
  min_master_version = data.google_container_engine_versions.main.release_channel_latest_version["REGULAR"]

  networking_mode = "VPC_NATIVE"
  ip_allocation_policy {}

  initial_node_count       = 1
  remove_default_node_pool = true

  node_config {
    machine_type = "e2-standard-2"
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  identity_service_config {
    enabled = false
  }

  addons_config {
    config_connector_config {
      enabled = false
    }
  }

  maintenance_policy {
    daily_maintenance_window {
      start_time = "03:00"
    }
    maintenance_exclusion {
      exclusion_name = "stop being so disruptive GKE"
      start_time     = timestamp()
      # end is 179 days / 4296h from now
      end_time = timeadd(timestamp(), "4296h")
      exclusion_options {
        scope = "NO_MINOR_OR_NODE_UPGRADES"
      }
    }
  }

  enable_tpu = false

  cluster_autoscaling {
    enabled             = true
    autoscaling_profile = "OPTIMIZE_UTILIZATION"

    auto_provisioning_defaults {
      oauth_scopes = [
        "https://www.googleapis.com/auth/logging.write",
        "https://www.googleapis.com/auth/monitoring",
        "https://www.googleapis.com/auth/devstorage.read_only",
        "https://www.googleapis.com/auth/compute"
      ]
      management {
        auto_upgrade = true
        auto_repair  = true
      }
      disk_size = 100
      disk_type = "pd-ssd"
      shielded_instance_config {
        enable_secure_boot          = true
        enable_integrity_monitoring = true
      }
    }

    resource_limits {
      resource_type = "cpu"
      minimum       = 0
      maximum       = 96
    }
    resource_limits {
      resource_type = "memory"
      minimum       = 0
      maximum       = 1048
    }
    resource_limits {
      resource_type = "nvidia-l4"
      minimum       = 0
      maximum       = 8
    }
    resource_limits {
      resource_type = "nvidia-tesla-t4"
      minimum       = 0
      maximum       = 4
    }
  }

  lifecycle {
    ignore_changes = [
      initial_node_count,
      maintenance_policy["maintenance_exclusion"]
    ]
  }
}

resource "google_container_node_pool" "main" {
  for_each       = local._node_pools
  name           = each.key
  cluster        = google_container_cluster.main.id
  node_locations = each.value.node_locations
  autoscaling {
    min_node_count  = 0
    max_node_count  = each.value.max_node_count
    location_policy = "ANY"
  }
  node_config {
    spot         = true
    machine_type = each.value.machine_type
    ephemeral_storage_local_ssd_config {
      local_ssd_count = each.value.local_ssd_count
    }
    guest_accelerator {
      type  = "nvidia-l4"
      count = try(each.value.guest_accelerator_count, 0)
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
