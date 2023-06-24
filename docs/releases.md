# Releases

Update `IMG` in `Makefile`.

```
IMG ?= docker.io/substratusai/controller-manager:v0.0.1-alpha
```

Generate install manifest.

```sh
make install/kubernetes/system.yaml
```
