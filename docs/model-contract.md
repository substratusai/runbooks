# Model Contract

## Repo

Model directory should contain a Dockerfile.

## Container Image

As a part of the container build process:

- Model artifacts (i.e. `model.pt`, `tokenizer.pt`) should be saved into `/model/saved/`.
- Workspace directory should be `/model/` (i.e. `WORKDIR /model`).

## Scripts

Must be located in `$PATH`:

- `serve.sh`
    * Loads model from disk (`/model/saved/`).
    * Run a webserver on port `8080`.
        * Endpoints:
            * GET `/docs`
                * Serves API Documentation.
            * POST `/generate`
                * Accepts a request body of `{"prompt":"<your-prompt>", "max_new_tokens": 100}`.
                * Responds with a body of `{"generation": "<your-prompt><completion>"}`.
- `train.sh`
    * Writes logs to STDOUT/STDERR and optionally to `/model/logs/`.
    * If notebooks are run, it saves the `.ipynb` files with output into `/model/logs/`.
    * The `DATASET_PATH` environment vairable will be provided.
    * Can load an existing model from `/model/saved/`.
    * Saves new trained model to `/model/trained/` (which will be copied into the new container's `/model/saved/` directory).
- `lab.sh`
    * Should start a Jupyter Lab environment.
    * Should serve on port `8888`.

## Directory Structure

```
/model/
  src/     # Model source code
  saved/   # Model artifacts from build jobs (to be loaded for inference)
  trained/ # Model artifacts from training job
  logs/    # Output of building/training jobs
```


