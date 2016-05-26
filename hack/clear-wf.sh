#!/bin/bash
etcdctl rm /registry/ThirdPartyResourceData/nerdalize.com/workflows/default/test-workflow
kubectl.sh delete jobs --all
cat examples/workflow.json | curl --data "@-" -H "Content-Type:application/json" http://localhost:8080/apis/nerdalize.com/v1alpha1/namespaces/default/workflows

/usr/bin/watch kubectl.sh get jobs --show-labels
