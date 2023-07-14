# Container Contracts

Substratus orchestrates machine learning operations without requiring any language-specific library dependencies. As long as a container satisfies the "contract", that container can be orchestrated with Substratus. These contracts are designed to impose as few requirements as possible.

## Any container Contract

The repo should contain a Dockerfile.

As a part of building the Dockerfile:

- Model artifacts (i.e. `model.pt`, `tokenizer.pt`) should be saved into `/model/saved/`.
- Workspace directory should be `/model/` (i.e. `WORKDIR /model`).
- `COMMAND` or `ENTRYPOINT` should be defined to execute the main purpose of the container. E.g. call `train.py` or `download-model.py`


### Scripts

Must be located in `$PATH`:

- `notebook.sh` for any container image used in Substratus
    * Should start a Jupyter Lab/Notebook environment.
    * Should serve on port `8888`.

### Directory Structure

```
/base-model/saved # Optionally, when a baseModel is specified in spec
/model/           # Working directory
  src/            # Model source code
  saved/          # Model artifacts from build jobs or training jobs (to be loaded for inference)
  logs/           # Output of building/training jobs for debugging purposes
```

### Environment Variables

Substratus provides params as environment variables to containers.

For example, the below Substratus Model spec will pass `abc: xyz`
to the container using the environment variable `PARAM_ABC=xyz`:
```yaml
spec:
  params: {abc: xyz}
```

Parameters get converted to environment variables using the following scheme:

`PARAM_{upper(param_key)}={param_value}`

In addition, Substratus has the following built-in environment variables:

| Environment Variable | Source                                     |
| -------------------- | ------------------------------------------ |
| `DATA_PATH`          | Dataset (`/data/` + `.spec.filename`)      |

## Dataset Contract

The repo should contain a Dockerfile.

- Workspace directory should be `/dataset/` (i.e. `WORKDIR /dataset`).

### Directory Structure

```sh
/dataset/  # Working directory
  src/     # Data loading source code
  logs/    # Output of data loading jobs for debugging purposes
```

### Environment Variables

The following parameters are communicated through environment variables when `load.sh` is called and should be taken into account.

| Environment Variable | Source                                     |
| -------------------- | ------------------------------------------ |
| `DATA_PATH`          | Dataset (`/data/` + `.spec.filename`)      |

