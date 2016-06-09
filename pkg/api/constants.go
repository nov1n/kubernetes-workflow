package api

import (
	k8sApiExtensions "k8s.io/kubernetes/pkg/apis/extensions"
)

const (
	Kind        = "Workflow"
	Version     = "v1alpha1"
	Group       = "nerdalize.com"
	Resource    = "workflow"
	Description = "An API endpoint for workflows"
	// WorkflowUIDLabel is a unique label for workflows, used as prefix.
	WorkflowUIDLabel = Group + "/workflow-uid"
)

var (
	Versions = []k8sApiExtensions.APIVersion{
		{Name: "v1alpha1"},
	}
)
