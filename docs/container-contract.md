# Container Contract

Substratus orchestrates machine learning operations without requiring any language-specific library dependencies. As long as a container satisfies the "contract", that container can be orchestrated with Substratus. These contracts are designed to impose as few requirements as possible.

## Workdir

The workdir MUST be set to `/content`.

```Dockerfile
WORKDIR /content
```

### Scripts

The following scripts MUST be located in `$PATH`. It is recommended that these scripts are stored in `/content/scripts`.

- `notebook.sh`
  * Required for Model, Dataset, and Notebook containers.
  * Starts a Jupyter Lab/Notebook environment.
  * Serve on port `8888`.
  * Respects the `NOTEBOOK_TOKEN` environment variable.

### Directory Structure

```
/content/         # Working directory.
  data/           # Location where Datasets will be mounted (reading and loading).
  src/            # Source code (*.py, *.ipynb) for loading, training, etc.
  logs/           # Output of building/training jobs for debugging purposes.
  model/          # Location to store the resulting model from loading or training.
  saved-model/    # Location where a previously saved model will be mounted.
```

### Parameters

Substratus provides params as environment variables to containers.

For example, the below Substratus Model spec will pass `abc: xyz`
to the container using the environment variable `PARAM_ABC=xyz`:

```yaml
spec:
  params: {abc: xyz}
```

Parameters get converted to environment variables using the following scheme:

`PARAM_{upper(param_key)}={param_value}`
