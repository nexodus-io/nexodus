#!/bin/bash

set -e

# remove all output after the line that says "## Usage"
cat docs/user-guide/nexctl.md | sed -n '/## Usage/q;p' > docs/user-guide/nexctl.md.tmp

printf "### Usage\n\n" >> docs/user-guide/nexctl.md.tmp

# generate the usage output
make dist/nexctl
echo '```text' >> docs/user-guide/nexctl.md.tmp
dist/nexctl -h | docker run -i --rm --name txt2man quay.io/nexodus/mock:latest txt2man -t nexctl | man -l - | cat >> docs/user-guide/nexctl.md.tmp
echo '```' >> docs/user-guide/nexctl.md.tmp

for subcmd in device invitation nexd organization user security-group; do
    printf "\n#### nexctl $subcmd\n\n" >> docs/user-guide/nexctl.md.tmp
    echo '```text' >> docs/user-guide/nexctl.md.tmp
    dist/nexctl ${subcmd} -h | docker run -i --rm --name txt2man quay.io/nexodus/mock:latest txt2man -t nexctl-${subcmd} | man -l - | cat >> docs/user-guide/nexctl.md.tmp
    echo '```' >> docs/user-guide/nexctl.md.tmp
done

mv docs/user-guide/nexctl.md.tmp docs/user-guide/nexctl.md
