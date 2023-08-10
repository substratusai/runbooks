# Development

## Control Plane

Create a GCP environment.

```sh
make dev-up-gcp
```

Run Substratus control plane locally.

```sh
make dev-run-gcp
```

Delete GCP infra.

```sh
make dev-down-gcp
```

TODO: Automate the cleanup of PVs... Don't forget to manually clean them up for now.

## Kubectl Plugins

### Install from source

You can test out the latest kubectl plugin by building from source directly:

```sh
go build ./kubectl/cmd/notebook &&
    mv notebook /usr/local/bin/kubectl-notebook ||
    sudo mv notebook /usr/local/bin/kubectl-notebook
go build ./kubectl/cmd/applybuild &&
    mv applybuild /usr/local/bin/kubectl-applybuild ||
    sudo mv applybuild /usr/local/bin/kubectl-applybuild
```

The `kubectl notebook` command depends on container-tools for live-syncing. The plugin will try
to download these tools from GitHub releases if they dont already exist with the right versions.

You can build the container-tools for development purposes using the following. NOTE: This is the default cache directory on a mac, this will be different on other machine types.

```sh
export NODE_ARCH=amd64

rm -rf /Users/$USER/Library/Caches/substratus
mkdir -p /Users/$USER/Library/Caches/substratus/container-tools/$NODE_ARCH
GOOS=linux GOARCH=$NODE_ARCH go build ./containertools/cmd/nbwatch
mv nbwatch /Users/$USER/Library/Caches/substratus/container-tools/$NODE_ARCH/

echo "development" > /Users/$USER/Library/Caches/substratus/container-tools/version.txt
```

### Install from release

Release binaries are created for most architectures when the repo is tagged.
Be aware that moving the binary to your PATH might fail due to permissions
(observed on mac). If it fails, the script will retry the `mv` with `sudo` and
prompt you for your password:

```sh
bash -c "$(curl -fsSL https://raw.githubusercontent.com/substratusai/substratus/main/install/scripts/install-kubectl-plugins.sh)"
```

If the plugin installed correctly, you should see it listed as a `kubectl plugin`:

```sh
kubectl plugin list 2>/dev/null | grep kubectl-notebook
```
