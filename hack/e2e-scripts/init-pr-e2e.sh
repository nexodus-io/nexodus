#!/bin/bash
# This is the script used for ec2-dev GH action in nexodus
set -x

function usage {
  echo "Usage: $0 [pr_number|main]"
}

function checkout_pr {
  local pr_number=$1
  # Use the gh command to check out the pull request
  gh pr checkout $pr_number
}

function build_project {
  # replace the current kind images with this branch
  make images
  make load-images
  make recreate-db
  sleep 100
}

# grab a fresh copy of the repo to avoid fast-forward issues
rm -rf /home/ubuntu/nexodus
git clone https://github.com/nexodus-io/nexodus.git

# enter the nexodus directory
cd /home/ubuntu/nexodus

# Parse command-line arguments
if [ $# -eq 0 ]; then
  # If no PR number is passed, checkout main
  git checkout main
elif [ $# -eq 1 ]; then
  PR_NUMBER=$1
else
  usage
  exit 1
fi

if [[ "$PR_NUMBER" =~ ^[0-9]+$ ]]; then
  checkout_pr $PR_NUMBER
elif [ "$PR_NUMBER" = "main" ]; then
  git checkout main
else
  usage
  exit 1
fi

build_project
