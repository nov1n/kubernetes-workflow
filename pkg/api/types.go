/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"path"

	"github.com/golang/glog"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
)

// Workflow object representing a single workflow
type Workflow struct {
	k8sApiUnv.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	k8sApi.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of the Workflow
	Spec WorkflowSpec `json:"spec,omitempty"`

	// Status contains the current status of the Workflow
	Status WorkflowStatus `json:"status,omitempty"`
}

// WorkflowList represents a list of Workflow objects
type WorkflowList struct {
	k8sApiUnv.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	k8sApiUnv.ListMeta `json:"metadata,omitempty"`

	// Items contains the list of Workflow objects
	Items []Workflow `json:"items"`
}

// WorkflowSpec contains the Workflow specification
type WorkflowSpec struct {
	// Standard object's metadata.
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	k8sApi.ObjectMeta `json:"metadata,omitempty"`

	// Maximum time before the Workflow may run before it is killed
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`

	// Steps maps step names to WorkflowStep objects for O(1) access by name
	Steps map[string]WorkflowStep `json:"steps,omitempty"`

	// Selector for jobs that belong to the Workflow
	JobsSelector *k8sApiUnv.LabelSelector `json:"jobsSelector,omitempty"`
}

// WorkflowStep represents a single step in the Workflow
type WorkflowStep struct {
	// JobTemplate contains the job specificaton that should be run in this step
	// Only one of externalRef and jobTemplate can be set
	JobTemplate *k8sBatch.JobTemplateSpec `json:"jobTemplate,omitempty"`

	// ExternalRef contains a reference to another schedulable resource
	// Only one of ExternalRef and JobTemplate can be set
	ExternalRef *k8sApi.ObjectReference `json:"externalRef,omitempty"`

	// Dependecies represent dependecies of the current workflow step
	Dependencies []string `json:"dependencies,omitempty"`
}

// WorkflowConditionType is a type for conditions describing a Workflow
type WorkflowConditionType string

// Possible conditions which may be present on a Workflow at a point in time
const (
	// WorkflowComplete means the workflow has completed its execution
	WorkflowComplete WorkflowConditionType = "Complete"
	// WorkflowFailed means the workflow has failed its execution
	WorkflowFailed WorkflowConditionType = "Failed"
)

// WorkflowCondition describes a condition on a workflow
type WorkflowCondition struct {
	// Type of workflow condition
	Type WorkflowConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status k8sApi.ConditionStatus `json:"status"`
	// Last time the condition was checked.
	LastProbeTime k8sApiUnv.Time `json:"lastProbeTime,omitempty"`
	// Last time the condition transited from one status to another.
	LastTransitionTime k8sApiUnv.Time `json:"lastTransitionTime,omitempty"`
	// (brief) reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// WorkflowStatus represents the current state/status of the Workflow
type WorkflowStatus struct {
	// Conditions represent the latest available observations of an object's current state.
	Conditions []WorkflowCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// StartTime represents time when the workflow was acknowledged by the Workflow controller
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// StartTime doesn't consider startime of `ExternalReference`
	StartTime *k8sApiUnv.Time `json:"startTime,omitempty"`

	// CompletionTime represents time when the workflow was completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTime *k8sApiUnv.Time `json:"completionTime,omitempty"`

	// Statuses represent status of different steps
	Statuses map[string]WorkflowStepStatus `json:"statuses"`
}

// WorkflowStepStatus represents the status of a single Workflow step
type WorkflowStepStatus struct {
	// Complete reports the completion of status
	Complete bool `json:"complete"`
	// Reference contains a reference to the WorkflowStep
	Reference k8sApi.ObjectReference `json:"reference"`
}

// GetObjectKind returns a type description of a Workflow
func (wf *Workflow) GetObjectKind() k8sApiUnv.ObjectKind {
	return &k8sApiUnv.TypeMeta{
		Kind:       Kind,
		APIVersion: path.Join(Group, Version),
	}
}

// GetObjectKind returns a type description of a WorkflowList
func (wf *WorkflowList) GetObjectKind() k8sApiUnv.ObjectKind {
	return &k8sApiUnv.TypeMeta{
		Kind:       Kind,
		APIVersion: path.Join(Group, Version),
	}
}

// setLabels adds the map to the workflow as labels.
func (wf *Workflow) setLabels(labels map[string]string) {
	if wf.Labels == nil {
		wf.Labels = make(map[string]string)
	}
	for key, value := range labels {
		wf.Labels[key] = value
	}
}

// SetDefaults initializes the workflow with default values.
func (wf *Workflow) SetDefaults() {
	wf.Status.Statuses = make(map[string]WorkflowStepStatus, len(wf.Spec.Steps))
	now := k8sApiUnv.Now()
	wf.Status.StartTime = &now
	wf.SetUID()
}

// SetUID sets the UID for a workflow.
func (wf *Workflow) SetUID() {
	glog.V(3).Infof("Setting labels on wf %v", wf.Name)
	if wf.Spec.JobsSelector == nil {
		wf.Spec.JobsSelector = &k8sApiUnv.LabelSelector{
			MatchLabels: make(map[string]string),
		}
	}
	wf.setLabels(map[string]string{
		WorkflowUIDLabel: string(wf.UID),
	})
	wf.Spec.JobsSelector.MatchLabels[WorkflowUIDLabel] = string(wf.UID)
}

// AddCompleteCondition add the Complete condition to the workflow.
func (wf *Workflow) AddCompleteCondition() {
	now := k8sApiUnv.Now()
	condition := WorkflowCondition{
		Type:               WorkflowComplete,
		Status:             k8sApi.ConditionTrue,
		LastProbeTime:      now,
		LastTransitionTime: now,
	}
	wf.Status.Conditions = append(wf.Status.Conditions, condition)
	wf.Status.CompletionTime = &now
}

// IsFinished returns whether a workflow is finished.
func (wf *Workflow) IsFinished() bool {
	for _, c := range wf.Status.Conditions {
		conditionWFFinished := (c.Type == WorkflowComplete || c.Type == WorkflowFailed)
		conditionTrue := c.Status == k8sApi.ConditionTrue
		if conditionWFFinished && conditionTrue {
			glog.V(3).Infof("Workflow %v finished", wf.Name)
			return true
		}
	}
	return false
}

// DependenciesResolved returns true when all the step's dependencies are
// completed or when the step has no dependencies.
func (s *WorkflowStep) DependenciesResolved(workflow *Workflow) bool {
	for _, dependencyName := range s.Dependencies {
		dependencyStatus, ok := workflow.Status.Statuses[dependencyName]
		if !ok || !dependencyStatus.Complete {
			return false
		}
	}
	return true
}
