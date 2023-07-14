# Releases

## Update Controller Image

Update `IMG` in `Makefile`.

```makefile
IMG ?= docker.io/substratusai/controller-manager:v0.0.1-alpha
```

Generate install manifest.

```sh
make install/kubernetes/system.yaml
```

## Generate snippets

We use [embedmd](https://github.com/campoy/embedmd) to help keep documentation
up to date with snippets. Run the following to generate new doc snippets:

```bash
make docs
```
