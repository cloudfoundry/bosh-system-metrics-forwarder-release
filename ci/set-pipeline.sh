#!/bin/bash

set -ex

lpass ls > /dev/null # check that we're logged in

fly -t superpipe set-pipeline \
    -p bosh-system-metrics \
    -c bosh-system-metrics.yml \
    -l <(lpass show --notes "Shared-apm/concourse/bosh-system-metrics-creds.yml") \
    -l scripts.yml
