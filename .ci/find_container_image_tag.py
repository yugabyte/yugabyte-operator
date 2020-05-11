#!/usr/bin/env python3
"""Python script to find container image tag from YugabyteDB release version"""
from optparse import OptionParser
from re import match
from requests import request
from sys import exit

REGISTRY_TAGS_URL = "https://registry.hub.docker.com/v1/repositories/yugabytedb/yugabyte/tags"


def main(release):
    """Use YugabyteDB release version to find respective container image tag"""
    if not match("[0-9]+.[0-9]+.[0-9]+.[0-9]+", release.version):
        exit("Version validation failed: required format: *.*.*.*")

    response = request(method='GET', url=REGISTRY_TAGS_URL)
    if not response.ok:
        exit("Got {} from {}.".format(response.status_code, REGISTRY_TAGS_URL))
    json_response = response.json()

    tags = dict()
    for tag_obj in json_response:
        tag = tag_obj['name']
        if tag.startswith(release.version):
            build_number = int(tag[tag.rindex("-b")+2:])
            tags[build_number] = tag
    if not tags:
        exit("Given version did not match any of the tags")

    latest_build = max(tags.keys())
    latest_tag = tags[latest_build]
    if not match("[0-9]+.[0-9]+.[0-9]+.[0-9]+-b[0-9]+", latest_tag):
        exit("Image tag '{}' is invalid, must be \
of the form N.N.N.N-bN".format(latest_tag))
    print(latest_tag)


if __name__ == "__main__":
    usage = "usage: %prog [options] -r <release-version>"
    parser = OptionParser(usage)
    parser.add_option("-r", "--release", type="string", dest="version")

    (ARGS, OPTIONS) = parser.parse_args()

    for option in ARGS.__dict__:
        if ARGS.__dict__[option] is None:
            parser.error("failed to parse the arguments.")
    main(ARGS)
