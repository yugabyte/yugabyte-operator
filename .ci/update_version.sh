#!/bin/bash

set -o errexit -o pipefail

# version_gt compares the given two versions.
# It returns 0 exit code if the version1 is greater than version2.
# https://web.archive.org/web/20191003081436/http://ask.xmodulo.com/compare-two-version-numbers.html
function version_gt() {
  test "$(echo -e "$1\n$2" | sort -V | head -n 1)" != "$1"
}


if [[ -z "$1" ]]; then
  echo "This script needs at least one argument, release_version is missing." 1>&2
  exit 1
fi

release_version="$1"
controller_go_file="pkg/controller/ybcluster/ybcluster_controller.go"
custom_resource_file="deploy/crds/yugabyte.com_v1alpha1_ybcluster_full_cr.yaml"

current_version_tag="$(grep -E \
                    '^[[:space:]]+imageTagDefault[[:space:]]+=[[:space:]]+".*"' \
                    "${controller_go_file}" | cut -d '"' -f "2")"
if ! version_gt "${release_version}" "${current_version_tag%-b*}" ; then
  echo "Release version is either older or equal to the current version: '${release_version}' <= '${current_version_tag%-b*}'" 1>&2
  exit 1
fi

# find container image tag respective to YugabyteDB release version
container_image_tag="$(python3 ".ci/find_container_image_tag.py" "-r" "${release_version}")"
echo "Latest container image tag for '${release_version}': '${container_image_tag}'"

# update the tag in Go source file
echo "Updating file '${controller_go_file}' with tag '${container_image_tag}'."
# sed: select the address range i.e. the line containing
# yugabyteDBImageName and then replace 'z.y.z.w-bn' string from that
# line https://unix.stackexchange.com/a/315082
sed -i -E "/^[[:space:]]+yugabyteDBImageName[[:space:]]+=.*$/ \
  s/[0-9]+.[0-9]+.[0-9]+.[0-9]+-b[0-9]+/${container_image_tag}/g" \
  "${controller_go_file}"
sed -i -E "/^[[:space:]]+imageTagDefault[[:space:]]+=.*$/ \
  s/[0-9]+.[0-9]+.[0-9]+.[0-9]+-b[0-9]+/${container_image_tag}/g" \
  "${controller_go_file}"

# update the tag in custom resource manifest file
echo "Updating file '${custom_resource_file}' with tag '${container_image_tag}'."
sed -i -E "/^[[:space:]]{4}tag: .*$/ \
  s/[0-9]+.[0-9]+.[0-9]+.[0-9]+-b[0-9]+/${container_image_tag}/g" \
  "${custom_resource_file}"
