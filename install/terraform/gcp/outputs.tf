output "cluster_name" {
  value = google_container_cluster.main.name
}

output "cluster_region" {
  value = google_container_cluster.main.location
}
