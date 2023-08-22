# Releases
Generating releases is done by doing the following:

1. Update the `VERSION` in Makefile

   ```makefile
    VERSION ?= v0.6.5-alpha
   ```

2. Generate the new install manifest that points to the new image

   ```sh
   make prepare-release
   ```

3. Submit a PR

4. Once the PR is merged make sure to create the new release by pushing a new tag with the exact same version name

   ```sh
   git tag v0.6.5-alpha
   git push --tags
   ```
