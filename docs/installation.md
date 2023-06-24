# Installation

The `install/` directory contains the cluster and infrastructure configuration needed to get Substratus up and running. All configuration is documented in delarative formats (`.yaml`, `.tf`, `Dockerfile`).

The base set of configurations are intended to work in a brand new cloud project free of any significant organizational policies. These configurations will need to be modified to fit within a typical enterprise's cloud environment.

## Directory structure

```
install/
  scripts/    # Helper scripts for streamlining the install process into single commands.
  terraform/  # Stands up a cluster and supporting infrastructure (such as buckets, image resgistries, etc.).
  kubernetes/ # Installs custom resources, controllers, etc. into a running cluster.
```

## Configuration Lookup

The Terraform and Kubernetes configurations do not attempt to export every option as a variable. In order to keep the configurations simple, most options are set directly in the `.tf` and `.yaml` files. You will likely need to adopt and modify these files for your environment. A few common configurations items are exposed at variables (`terraform.tfvars` for Terraform, and `kind: ConfigMap` for Kubernetes).

| Configuration | File                                     |
| ------------- | ---------------------------------------- |
| Project ID    | `scripts/gcp-up.sh`                      |
| Region/Zone   | `terraform/terraform.tfvars`             |
| GPU Types     | `kubernetes/config.yaml`                 |
