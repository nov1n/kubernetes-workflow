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

package controller

import (
	"fmt"
	"sync"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sValidation "k8s.io/kubernetes/pkg/api/validation"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
	k8sClSet "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	k8sRec "k8s.io/kubernetes/pkg/client/record"
	k8sCtl "k8s.io/kubernetes/pkg/controller"
	k8sLabels "k8s.io/kubernetes/pkg/labels"
	k8sRunt "k8s.io/kubernetes/pkg/runtime"
)

// JobControlInterface defines methods for JobControl
type JobControlInterface interface {
	CreateJob(namespace string, template *k8sBatch.JobTemplateSpec, object k8sRunt.Object, key string) error
	DeleteJob(namespace, name string, object k8sRunt.Object) error
}

// WorkflowJobControl is the workflow implementation of JobControlInterface
type WorkflowJobControl struct {
	KubeClient k8sClSet.Interface
	Recorder   k8sRec.EventRecorder
}

var _ JobControlInterface = &WorkflowJobControl{}

// getJobsPrefix returns the prefix used for controller names
func getJobsPrefix(controllerName string) string {
	prefix := fmt.Sprintf("%s-", controllerName)
	if errs := k8sValidation.NameIsDNSSubdomain(prefix, true); errs != nil {
		prefix = controllerName
	}
	return prefix
}

// getJobsAnnotationSet returns the set of annotations for a given JobTemplateSpec in a workflow
func getJobsAnnotationSet(template *k8sBatch.JobTemplateSpec, object k8sRunt.Object) (k8sLabels.Set, error) {
	workflow := *object.(*api.Workflow)
	desiredAnnotations := make(k8sLabels.Set)
	for k, v := range workflow.Annotations {
		desiredAnnotations[k] = v
	}
	createdByRef, err := k8sApi.GetReference(object)
	if err != nil {
		return desiredAnnotations, fmt.Errorf("unable to get controller reference: %v", err)
	}

	//TODO: codec  hardcoded to v1 for the moment.
	codec := k8sApi.Codecs.LegacyCodec(k8sApiUnv.GroupVersion{Group: k8sApi.GroupName, Version: "v1"})

	createdByRefJSON, err := k8sRunt.Encode(codec, &k8sApi.SerializedReference{
		Reference: *createdByRef,
	})
	if err != nil {
		return desiredAnnotations, fmt.Errorf("unable to serialize controller reference: %v", err)
	}
	desiredAnnotations[k8sCtl.CreatedByAnnotation] = string(createdByRefJSON)
	return desiredAnnotations, nil
}

// WorkflowStepLabelKey defines the label assigned to a workflow
const WorkflowStepLabelKey = "kubernetes.io/workflow"

// getWorkflowJobLabelSet returns the set of labels for a job in a workflow
func getWorkflowJobLabelSet(workflow *api.Workflow, template *k8sBatch.JobTemplateSpec, stepName string) k8sLabels.Set {
	desiredLabels := make(k8sLabels.Set)
	for k, v := range workflow.Labels {
		desiredLabels[k] = v
	}
	for k, v := range template.Labels {
		desiredLabels[k] = v
	}
	desiredLabels[WorkflowStepLabelKey] = stepName // @sdminonne: TODO double check this
	return desiredLabels
}

// CreateWorkflowJobLabelSelector creates a label selector from the label set for a job in a workflow
func CreateWorkflowJobLabelSelector(workflow *api.Workflow, template *k8sBatch.JobTemplateSpec, stepName string) k8sLabels.Selector {
	return k8sLabels.SelectorFromSet(getWorkflowJobLabelSet(workflow, template, stepName))
}

// CreateJob creates a job in a workflow
func (w WorkflowJobControl) CreateJob(namespace string, template *k8sBatch.JobTemplateSpec, object k8sRunt.Object, stepName string) error {
	workflow := object.(*api.Workflow)
	desiredLabels := getWorkflowJobLabelSet(workflow, template, stepName)
	desiredAnnotations, err := getJobsAnnotationSet(template, object)
	if err != nil {
		return err
	}
	meta, err := k8sApi.ObjectMetaFor(object)
	if err != nil {
		return fmt.Errorf("object does not have ObjectMeta, %v", err)
	}
	prefix := getJobsPrefix(meta.Name)
	job := &k8sBatch.Job{
		ObjectMeta: k8sApi.ObjectMeta{
			Labels:       desiredLabels,
			Annotations:  desiredAnnotations,
			GenerateName: prefix,
		},
	}

	if err := k8sApi.Scheme.Convert(&template.Spec, &job.Spec); err != nil {
		return fmt.Errorf("unable to convert job template: %v", err)
	}

	newJob, err := w.KubeClient.Batch().Jobs(namespace).Create(job)
	if err != nil {
		w.Recorder.Eventf(object, k8sApi.EventTypeWarning, "FailedCreate", "Error creating: %v", err)
		return fmt.Errorf("unable to create job: %v", err)
	}
	glog.V(3).Infof("Controller %v created job %v", meta.Name, newJob.Name)
	return nil
}

// DeleteJob deletes a job TODO: Implement this
func (w WorkflowJobControl) DeleteJob(namespace, jobName string, object k8sRunt.Object) error {
	return nil
}

// IsJobFinished returns whether a job finished or not
func IsJobFinished(j *k8sBatch.Job) bool {
	for _, c := range j.Status.Conditions {
		conditionJobFinished := c.Type == k8sBatch.JobComplete || c.Type == k8sBatch.JobFailed
		conditionTrue := c.Status == k8sApi.ConditionTrue
		if conditionJobFinished && conditionTrue {
			return true
		}
	}
	return false
}

// FakeJobControl mocks a job control for testing purposes
type FakeJobControl struct {
	sync.Mutex
	CreatedJobTemplates []k8sBatch.JobTemplateSpec
	DeletedJobNames     []string
	Err                 error
}

var _ JobControlInterface = &FakeJobControl{}

// CreateJob simulates creating a job
func (f *FakeJobControl) CreateJob(namespace string, template *k8sBatch.JobTemplateSpec, object k8sRunt.Object, key string) error {
	f.Lock()
	defer f.Unlock()
	f.CreatedJobTemplates = append(f.CreatedJobTemplates, *template)
	return nil
}

// DeleteJob simulates deleting a job
func (f *FakeJobControl) DeleteJob(namespace, name string, object k8sRunt.Object) error {
	f.Lock()
	defer f.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.DeletedJobNames = append(f.DeletedJobNames, name)
	return nil
}
