#
# SPDX-License-Identifier: Apache-2.0
#

ROOT_DIR := $(dir $(lastword $(MAKEFILE_LIST)))../../
include $(ROOT_DIR)podvm/Makefile.inc

.PHONY: image clean

image: $(IMAGE_FILE)

$(IMAGE_FILE): $(BINARIES) $(FILES)
	pwd
	echo "mgns root_dir: $(ROOT_DIR)"
	echo "mgns ubuntu_amd64_IMAGE_URL: $(ubuntu_amd64_IMAGE_URL)"

clean:
	rm -f "$(IMAGE_FILE)" "$(UBUNTU_IMAGE_FILE)" $(BINARIES)
	rm -fr "$(SKOPEO_SRC)" "$(UMOCI_SRC)" "$(PAUSE_SRC)" "$(FILES_DIR)/$(PAUSE_BUNDLE)"

.PHONY: force
force:
