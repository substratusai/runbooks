# Container Contracts

Substratus orchestrates machine learning operations without requiring any language-specific library dependencies. As long as a container satisfies the "contract", that container can be orchestrated with Substratus. These contracts are designed to impose as few requirements as possible.

## Any container Contract

The repo should contain a Dockerfile.

As a part of building the Dockerfile:

- Model artifacts (i.e. `model.pt`, `tokenizer.pt`) should be saved into `/model/saved/`.
- Workspace directory should be `/model/` (i.e. `WORKDIR /model`).

### Scripts

Must be located in `$PATH`:

- `load.sh` for Model that specifies loader section
    * Loads a model from an external source to (`/model/saved`)
- `serve.sh` for ModelServer container image
    * Loads model from disk (`/model/saved/`).
    * Run a webserver on port `8080`. Right now, there is no contract on the specific endpoints to serve but this could change in the future
- `train.sh` for Model that specifies trainer section
    * Writes logs to STDOUT/STDERR and optionally to `/model/logs/`.
    * If notebooks are run, it saves the `.ipynb` files with output into `/model/logs/`.
    * The `DATASET_PATH` environment vairable will be provided.
    * Can load an existing model from `/model/saved/`.
    * Saves new trained model to `/model/trained/` (which will be copied into the new container's `/model/saved/` directory).
- `notebook.sh` for any container image used in Substratus
    * Should start a Jupyter Lab/Notebook environment.
    * Should serve on port `8888`.

### Directory Structure

```
/model/    # Working directory
  src/     # Model source code
  saved/   # Model artifacts from build jobs (to be loaded for inference)
  trained/ # Model artifacts from training job
  logs/    # Output of building/training jobs for debugging purposes
```

### Environment Variables

The following parameters are communicated through environment variables when `train.sh` is called and should be taken into account.

| Environment Variable | Source                                     |
| -------------------- | ------------------------------------------ |
| `TRAIN_DATA_PATH`    | Dataset (`/data/` + `.spec.filename`)      |
| `TRAIN_DATA_LIMIT`   | Model (`.spec.training.params.dataLimit`)  |
| `TRAIN_BATCH_SIZE`   | Model (`.spec.training.params.batchSize`)  |
| `TRAIN_EPOCHS`       | Model (`.spec.training.params.epochs`)     |

## Dataset Contract

The repo should contain a Dockerfile.

- Workspace directory should be `/dataset/` (i.e. `WORKDIR /dataset`).

### Scripts

Must be located in `$PATH`:

- `load.sh`
    * Saves data to disk in the file path specified by the `LOAD_DATA_PATH` environment variable.
    * Writes logs to STDOUT/STDERR and optionally to `/dataset/logs/`.
    * If notebooks are run, it saves the `.ipynb` files with output into `/dataset/logs/`.
- `notebook.sh`
    * Should start a Jupyter Lab/Notebook environment.
    * Should serve on port `8888`.

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
| `LOAD_DATA_PATH`     | Dataset (`/data/` + `.spec.filename`)      |

