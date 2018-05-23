#!/bin/sh

: "${CI_API_V4_URL:="https://gitlab.com/api/v4"}"
: "${CI_PROJECT_ID:="10755780"}"
: "${CI_COMMIT_SHA:="$(git rev-parse master)"}"
: "${API_TOKEN:?"please set this environment variable to a GitLab private token with API access"}"

running_master_pipeline_id() {
    url="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines?sha=${CI_COMMIT_SHA}"
    json="$(curl -s -H "Private-Token: ${API_TOKEN}" "${url}")"
    echo "$json" | jq '.[]|select(.ref == "master" and .status == "running")|.id'
}

running="$(running_master_pipeline_id)"
while [ -n "$running" ]; do
  echo "Waiting for pipeline $running to finish...";
  sleep 10;
  running="$(running_master_pipeline_id)"
done
