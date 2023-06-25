data "google_container_engine_versions" "main" {
  provider = google-beta
  location = var.region
}

resource "google_container_cluster" "main" {
  provider = google-beta

  name    = local.name
  project = var.project_id

  location           = var.region
  min_master_version = data.google_container_engine_versions.main.release_channel_latest_version["STABLE"]
  release_channel {
    channel = "STABLE"
  }

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
    gcs_fuse_csi_driver_config {
      enabled = true
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
    # from https://cloud.google.com/compute/docs/gpus/#nvidia_gpus_for_compute_workloads
    # TODO(bjb): ideally set these to align with regional quota on the project
    resource_limits {
      resource_type = "nvidia-l4"
      minimum       = 0
      maximum       = 8
    }
    resource_limits {
      resource_type = "nvidia-tesla-a100" # 40gb
      minimum       = 0
      maximum       = 8
    }
    resource_limits {
      resource_type = "nvidia-a100-80gb"
      minimum       = 0
      maximum       = 8
    }
    resource_limits {
      resource_type = "nvidia-tesla-t4"
      minimum       = 0
      maximum       = 8
    }
    resource_limits {
      resource_type = "nvidia-tesla-v100"
      minimum       = 0
      maximum       = 8
    }
    resource_limits {
      resource_type = "nvidia-tesla-p100"
      minimum       = 0
      maximum       = 8
    }
    resource_limits {
      resource_type = "nvidia-tesla-p4"
      minimum       = 0
      maximum       = 8
    }
    # EOL on GCP May 1, 2024: https://cloud.google.com/compute/docs/eol/k80-eol
    resource_limits {
      resource_type = "nvidia-tesla-k80"
      minimum       = 0
      maximum       = 8
    }
  }

  lifecycle {
    ignore_changes = [
      initial_node_count,
      node_config,
      maintenance_policy["maintenance_exclusion"]
    ]
  }
}
