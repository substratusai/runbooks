# Installation

The `install/` directory contains the cluster and infrastructure configuration needed to get Substratus up and running. The base set of configurations are intended to work in a brand new cloud project free of any significant organizational policies. These configurations will need to be modified to fit within a typical enterprise's cloud environment.

## Directory structure

```
install/
  scripts/    # Helper scripts for streamlining the install process into single commands.
  terraform/  # Stands up a cluster and supporting infrastructure (such as buckets, image resgistries, etc.).
  kubernetes/ # Installs custom resources, controllers, etc. into a running cluster.
```

## Configuration Lookup

| Configuration | File                                     |
| ------------- | ---------------------------------------- |
| Region/Zone   | `terraform/terraform.tfvars`             |
| GPU Types     | `kubernetes/config.yaml`                 |
