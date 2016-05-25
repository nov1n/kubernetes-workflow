#!/bin/bash

# Remove previous etcd entry about the test-workflow
etcdctl rm /registry/ThirdPartyResourceData/nerdalize.com/workflows/default/test-workflow

# Delete all previous jobs if any
kubectl.sh delete jobs --all

# Submit the workflow manifest to the kubernetes API server
cat examples/workflow.json | curl --data "@-" -H "Content-Type:application/json" http://localhost:8080/apis/nerdalize.com/v1alpha1/namespaces/default/workflows

# Watch jobs for changes
/usr/bin/watch kubectl.sh get jobs --show-labels
