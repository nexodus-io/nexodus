#!/bin/bash

set -e

# remove all output after the line that says "## Usage"
cat docs/user-guide/nexd.md | sed -n '/## Usage/q;p' > docs/user-guide/nexd.md.tmp

printf "### Usage\n\n" >> docs/user-guide/nexd.md.tmp

# generate the usage output
echo '```text' >> docs/user-guide/nexd.md.tmp
dist/nexd -h >> docs/user-guide/nexd.md.tmp
echo '```' >> docs/user-guide/nexd.md.tmp

for subcmd in proxy router relay; do
    printf "\n#### nexd $subcmd\n\n" >> docs/user-guide/nexd.md.tmp
    echo '```text' >> docs/user-guide/nexd.md.tmp
    dist/nexd ${subcmd} -h >> docs/user-guide/nexd.md.tmp
    echo '```' >> docs/user-guide/nexd.md.tmp
done

mv docs/user-guide/nexd.md.tmp docs/user-guide/nexd.md
