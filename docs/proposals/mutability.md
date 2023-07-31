# Mutability in Substratus

This is a working design doc, not the current implementation.

## Models

## Datasets

## Servers

```yaml
kind: Server

spec:

  params: {} # Change to field triggers redeploy.

  image: "" # Change to field triggers redeploy.

  build:
    upload:
      md5Checksum: "" # Change to field triggers rebuild.
    git:
      repo: ""    # Change to field triggers rebuild.

      tag: ""     # Same as branch:
      branch: ""  # Change to field triggers rebuild.
                  # Change in git reported in .status.image.synced = false ...
                  # Rebuild happens automatically.

      commit: ""  # Change to field triggers rebuild. Git not monitored for changes.

status:

  image:
    synced: true/false # Reports whether the .image.name is in sync with the source (.git/.upload).
```

## Notebooks

```yaml
kind: Notebook

spec:

  params: {} # Change to field triggers redeploy.

  image: "" # Change to field triggers redeploy.

  build:
    upload:
      md5Checksum: "" # Change to field triggers rebuild.
    git:
      repo: ""    # Change to field triggers rebuild.

      tag: ""     # Same as branch:
      branch: ""  # Change to field triggers rebuild.
                  # Change in git reported in .status.image.synced = false ...
                  # User prompted in CLI/UI to decide if they want to rebuild.

      commit: ""  # Change to field triggers rebuild. Git not monitored for changes.

status:

  image:
    synced: true/false # Reports whether the .image.name is in sync with the source (.git/.upload).
```
