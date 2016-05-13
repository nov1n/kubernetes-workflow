package api

import (
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnversioned "k8s.io/kubernetes/pkg/api/unversioned"
)

// Workflow is a collection of steps that can have dependencies.
type Workflow struct {
	k8sApiUnversioned.TypeMeta `json:",inline"`
	k8sApi.ObjectMeta          `json:"metadata,omitempty"`

	Spec WorkflowSpec `json:"spec,omitempty"`
}

// WorkflowSpec is a description of a Workflow.
type WorkflowSpec struct {
	Steps []WorkflowStep `json:"steps"`
}

// WorkflowStep is a step in a Workflow.
type WorkflowStep struct {
	Name      string `json:"name"`
	DependsOn string `json:"dependsOn"`
}

// WorkflowList is a list of Workflows.
type WorkflowList struct {
	k8sApiUnversioned.TypeMeta `json:",inline"`
	k8sApiUnversioned.ListMeta `json:"metadata,omitempty"`

	Items []Workflow `json:"items"`
}

func (wf *Workflow) GetObjectKind() k8sApiUnversioned.ObjectKind {
	return &k8sApiUnversioned.TypeMeta{
		Kind:       "Workflow",
		APIVersion: "nerdalize.com/v1alpha1",
	}
}
func (wf *WorkflowList) GetObjectKind() k8sApiUnversioned.ObjectKind {
	return &k8sApiUnversioned.TypeMeta{
		Kind:       "Workflow",
		APIVersion: "nerdalize.com/v1alpha1",
	}
}
