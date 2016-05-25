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

package validation

import (
	"fmt"
	"sort"

	"github.com/nov1n/kubernetes-workflow/pkg/api"

	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sValidationUnv "k8s.io/kubernetes/pkg/api/unversioned/validation"
	k8sValidation "k8s.io/kubernetes/pkg/api/validation"
	k8sSets "k8s.io/kubernetes/pkg/util/sets"
	k8sField "k8s.io/kubernetes/pkg/util/validation/field"
)

// ValidateWorkflow validates a workflow
func ValidateWorkflow(workflow *api.Workflow) k8sField.ErrorList {
	// Workflows and rcs have the same name validation
	allErrs := k8sValidation.ValidateObjectMeta(&workflow.ObjectMeta, true, k8sValidation.NameIsDNSSubdomain, k8sField.NewPath("metadata"))
	allErrs = append(allErrs, ValidateWorkflowSpec(&workflow.Spec, k8sField.NewPath("spec"))...)
	return allErrs
}

// ValidateWorkflowSpec validates a WorkflowSpec returning errors in case it is invalid
func ValidateWorkflowSpec(spec *api.WorkflowSpec, fieldPath *k8sField.Path) k8sField.ErrorList {
	allErrs := k8sField.ErrorList{}

	if spec.ActiveDeadlineSeconds != nil {
		allErrs = append(allErrs, k8sValidation.ValidateNonnegativeField(int64(*spec.ActiveDeadlineSeconds), fieldPath.Child("activeDeadlineSeconds"))...)
	}
	if spec.JobsSelector == nil {
		allErrs = append(allErrs, k8sField.Required(fieldPath.Child("jobsSelector"), ""))
	} else {
		allErrs = append(allErrs, k8sValidationUnv.ValidateLabelSelector(spec.JobsSelector, fieldPath.Child("jobsSelector"))...)
	}
	allErrs = append(allErrs, ValidateWorkflowSteps(spec.Steps, fieldPath.Child("steps"))...)
	return allErrs
}

// topologicalSort sorts a map of WorkflowSteps to check for cyclicity
func topologicalSort(steps map[string]api.WorkflowStep, fieldPath *k8sField.Path) ([]string, *k8sField.Error) {
	sorted := make([]string, len(steps))
	temporary := map[string]bool{}
	permanent := map[string]bool{}
	cycle := []string{}
	isCyclic := false
	cycleStart := ""
	var visit func(string) *k8sField.Error
	visit = func(n string) *k8sField.Error {
		if _, found := steps[n]; !found {
			return k8sField.NotFound(fieldPath, n)
		}
		switch {
		case temporary[n]:
			isCyclic = true
			cycleStart = n
			return nil
		case permanent[n]:
			return nil
		}
		temporary[n] = true
		for _, m := range steps[n].Dependencies {
			if err := visit(m); err != nil {
				return err
			}
			if isCyclic {
				if len(cycleStart) != 0 {
					cycle = append(cycle, n)
					if n == cycleStart {
						cycleStart = ""
					}
				}
				return nil
			}
		}
		delete(temporary, n)
		permanent[n] = true
		sorted = append(sorted, n)
		return nil
	}
	var keys []string
	for k := range steps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if permanent[k] {
			continue
		}
		if err := visit(k); err != nil {
			return nil, err
		}
		if isCyclic {
			return nil, k8sField.Forbidden(fieldPath, fmt.Sprintf("detected cycle %s", cycle))
		}
	}
	return sorted, nil
}

// ValidateWorkflowSteps validates the steps in a workflow returning errors in case it is invalid
func ValidateWorkflowSteps(steps map[string]api.WorkflowStep, fieldPath *k8sField.Path) k8sField.ErrorList {
	allErrs := k8sField.ErrorList{}
	if _, err := topologicalSort(steps, fieldPath); err != nil {
		allErrs = append(allErrs, err)
	}
	for k, v := range steps {
		if v.JobTemplate != nil && v.ExternalRef != nil {
			allErrs = append(allErrs, k8sField.Invalid(fieldPath, k, "jobTemplate and externalRef are mutually exclusive"))
		}
	}
	return allErrs
}

// ValidateWorkflowStatus validates the status of a workflow returning errors in case it is invalid
func ValidateWorkflowStatus(status *api.WorkflowStatus, fieldPath *k8sField.Path) k8sField.ErrorList {
	allErrs := k8sField.ErrorList{}

	if status.Statuses == nil {
		allErrs = append(allErrs, k8sField.Invalid(fieldPath.Child("statuses"), nil, "statuses map may not be nil"))
	}

	return allErrs
}

// getWorkflowRunningAndCompletedSteps returns two maps, one containing the running steps,
// the other containing the completed steps in a workflow
func getWorkflowRunningAndCompletedSteps(workflow *api.Workflow) (running, completed map[string]bool) {
	running = make(map[string]bool)
	completed = make(map[string]bool)
	if workflow.Status.Statuses == nil {
		return
	}
	for key := range workflow.Spec.Steps {
		if step, found := workflow.Status.Statuses[key]; found {
			if step.Complete {
				completed[key] = true
			} else {
				running[key] = true
			}
		}
	}
	return
}

// ValidateWorkflowUpdate validates a workflow update checking if it should be allowed
// It returns a list of errors in case validation fails
func ValidateWorkflowUpdate(workflow, oldWorkflow *api.Workflow) k8sField.ErrorList {
	allErrs := k8sValidation.ValidateObjectMetaUpdate(&workflow.ObjectMeta, &oldWorkflow.ObjectMeta, k8sField.NewPath("metadata"))

	runningSteps, completedSteps := getWorkflowRunningAndCompletedSteps(oldWorkflow)
	allCompleted := true
	for k := range oldWorkflow.Spec.Steps {
		if !completedSteps[k] {
			allCompleted = false
			break
		}
	}
	if allCompleted {
		allErrs = append(allErrs, k8sField.Forbidden(k8sField.NewPath("workflow"), "cannot update completed workflow"))
		return allErrs
	}

	allErrs = append(allErrs, ValidateWorkflowSpecUpdate(&workflow.Spec, &oldWorkflow.Spec, runningSteps, completedSteps, k8sField.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateWorkflowStatusUpdate(&workflow.Status, &oldWorkflow.Status, k8sField.NewPath("status"))...)
	return allErrs
}

// ValidateWorkflowSpecUpdate validates a spec update checking if it should be allowed
// It returns a list of errors in case validation fails
func ValidateWorkflowSpecUpdate(spec, oldSpec *api.WorkflowSpec, running, completed map[string]bool, fieldPath *k8sField.Path) k8sField.ErrorList {
	allErrs := k8sField.ErrorList{}
	allErrs = append(allErrs, ValidateWorkflowSpec(spec, fieldPath)...)
	allErrs = append(allErrs, k8sValidation.ValidateImmutableField(spec.JobsSelector, oldSpec.JobsSelector, fieldPath.Child("jobsSelector"))...)

	newSteps := k8sSets.NewString()
	for k := range spec.Steps {
		newSteps.Insert(k)
	}

	oldSteps := k8sSets.NewString()
	for k := range oldSpec.Steps {
		oldSteps.Insert(k)
	}

	removedSteps := oldSteps.Difference(newSteps)
	for _, s := range removedSteps.List() {
		if running[s] {
			allErrs = append(allErrs, k8sField.Forbidden(k8sField.NewPath("spec", "steps"), "cannot delete running step \""+s+"\""))
			return allErrs
		}
		if completed[s] {
			allErrs = append(allErrs, k8sField.Forbidden(k8sField.NewPath("spec", "steps"), "cannot delete completed step \""+s+"\""))
			return allErrs
		}
	}
	for k, v := range spec.Steps {
		if !k8sApi.Semantic.DeepEqual(v, oldSpec.Steps[k]) {
			if running[k] {
				allErrs = append(allErrs, k8sField.Forbidden(k8sField.NewPath("spec", "steps"), "cannot modify running step \""+k+"\""))
			}
			if completed[k] {
				allErrs = append(allErrs, k8sField.Forbidden(k8sField.NewPath("spec", "steps"), "cannot modify completed step \""+k+"\""))
			}
		}
	}
	return allErrs
}

// ValidateWorkflowStatusUpdate validates a status update checking if it should be allowed
// It returns a list of errors in case validation fails
func ValidateWorkflowStatusUpdate(status, oldStatus *api.WorkflowStatus, fieldPath *k8sField.Path) k8sField.ErrorList {
	allErrs := k8sField.ErrorList{}
	allErrs = append(allErrs, ValidateWorkflowStatus(status, fieldPath)...)

	return allErrs
}
