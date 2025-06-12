# copy-image

`copy-image` exports a community gallery image version into your own Shared Image Gallery.
The tool creates a temporary managed disk and managed image from the source community image
and publishes an image version under the specified gallery and image definition.
Temporary resources are removed automatically when the command completes.
## Building

Run `make azure-copy-image` from the `src/cloud-api-adaptor` directory to build the binary.

## Usage

```bash
copy-image --resource-group <name> --gallery-name <gallery> \
  --definition-name <definition> --community-image-id <community image version id>
```

The subscription ID and region are taken from the current Azure CLI configuration
if not explicitly provided with `--subscription-id` or `--location`.

### Examples

Copy a community image into gallery `my_gallery` and create version `0.0.1`:

```bash
copy-image -resource-group myrg \
  -gallery-name my_gallery \
  -definition-name my-def-cvm \
  -version-name 0.0.1 \
  -community-image-id /CommunityGalleries/...../Versions/0.14.0
```

Select target regions explicitly:

```bash
copy-image -resource-group myrg \
  -gallery-name my_gallery \
  -definition-name my-def-cvm \
  -target-regions westeurope,eastus \
  -community-image-id /CommunityGalleries/....
```

Environment variables `AZURE_SUBSCRIPTION_ID` and `AZURE_REGION` can also be
used to override the defaults.
