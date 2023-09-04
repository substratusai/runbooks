# Container Contract

Substratus orchestrates machine learning operations without requiring any language-specific library dependencies. As long as a container satisfies the "contract", that container can be orchestrated with Substratus. This contract is designed to impose as few requirements as possible.

## Workdir

The working directory MUST be set to `/content`.

```Dockerfile
WORKDIR /content
```

## Jupyter

This requirement applies to Model, Dataset, and Notebook containers.

The `notebook.sh` script MUST be located in `$PATH`. It is recommended that this script is stored in `/content/scripts`.

* Starts a Jupyter Lab/Notebook environment.
* Serve on port `8888`.
* Respects the `NOTEBOOK_TOKEN` environment variable.

Note: This requirement is satisfied by default when using Substratus base images.

## Directory Structure

```
/content/         # Working directory.
  data/           # Location where a previously stored Datasets is mounted.
  model/          # Location where a previously stored Model is mounted.
  output/         # Location to store output of a run.
  src/            # Source code (*.py, *.ipynb) for loading, training, etc.
```

## Parameters

Substratus provides params as a file (`/content/params.json`) and as environment variables to containers.

For example, the below Substratus Model spec will pass `abc: xyz`
to the container using the environment variable `PARAM_ABC=xyz`:

```yaml
spec:
  params: {abc: xyz}
```

Parameters get converted to environment variables using the following scheme.

`PARAM_{upper(param_key)}={param_value}`

## Server

Substratus Server containers are expected to:

* Serve HTTP traffic on port `8080`.
* Serve a 200 OK on the root path `/` when ready to serve traffic.
