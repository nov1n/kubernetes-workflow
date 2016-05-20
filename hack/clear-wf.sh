#!/bin/bash
etcdctl rm /registry/ThirdPartyResourceData/nerdalize.com/workflows/default/test-workflow
$PROJECTS/go/src/k8s.io/kubernetes/cluster/kubectl.sh delete jobs --all
cat $PROJECTS/go/src/github.com/nov1n/kubernetes-workflow/examples/workflow.json | curl --data "@-" -H "Content-Type:application/json" http://localhost:8080/apis/nerdalize.com/v1alpha1/namespaces/default/workflows

/usr/bin/watch $PROJECTS/go/src/k8s.io/kubernetes/cluster/kubectl.sh get jobs --show-labels
