package controller

import (
	"fmt"
	"sync"

	"github.com/golang/glog"
	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/apis/batch"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/record"
	k8sController "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
)

type JobControlInterface interface {
	// CreateJob
	CreateJob(namespace string, template *batch.JobTemplateSpec, object runtime.Object, key string) error
	// DeleteJob
	DeleteJob(namespace, name string, object runtime.Object) error
}

// RealJobControl is the default implementation of JobControlInterface
type WorkflowJobControl struct {
	KubeClient clientset.Interface
	Recorder   record.EventRecorder
}

var _ JobControlInterface = &WorkflowJobControl{}

func getJobsPrefix(controllerName string) string {
	prefix := fmt.Sprintf("%s-", controllerName)
	if ok, _ := validation.ValidateReplicationControllerName(prefix, true); !ok {
		prefix = controllerName
	}
	return prefix
}

func getJobsAnnotationSet(template *batch.JobTemplateSpec, object runtime.Object) (labels.Set, error) {
	workflow := *object.(*api.Workflow)
	desiredAnnotations := make(labels.Set)
	for k, v := range workflow.Annotations {
		desiredAnnotations[k] = v
	}
	createdByRef, err := k8sApi.GetReference(object)
	if err != nil {
		return desiredAnnotations, fmt.Errorf("unable to get controller reference: %v", err)
	}

	//TODO: codec  hardcoded to v1 for the moment.
	codec := k8sApi.Codecs.LegacyCodec(unversioned.GroupVersion{Group: k8sApi.GroupName, Version: "v1"})

	createdByRefJson, err := runtime.Encode(codec, &k8sApi.SerializedReference{
		Reference: *createdByRef,
	})
	if err != nil {
		return desiredAnnotations, fmt.Errorf("unable to serialize controller reference: %v", err)
	}
	desiredAnnotations[k8sController.CreatedByAnnotation] = string(createdByRefJson)
	return desiredAnnotations, nil
}

const WorkflowStepLabelKey = "kubernetes.io/workflow"

func getWorkflowJobLabelSet(workflow *api.Workflow, template *batch.JobTemplateSpec, stepName string) labels.Set {
	desiredLabels := make(labels.Set)
	for k, v := range workflow.Labels {
		desiredLabels[k] = v
	}
	for k, v := range template.Labels {
		desiredLabels[k] = v
	}
	desiredLabels[WorkflowStepLabelKey] = stepName // @sdminonne: TODO double check this
	return desiredLabels
}
func CreateWorkflowJobLabelSelector(workflow *api.Workflow, template *batch.JobTemplateSpec, stepName string) labels.Selector {
	return labels.SelectorFromSet(getWorkflowJobLabelSet(workflow, template, stepName))
}

func (w WorkflowJobControl) CreateJob(namespace string, template *batch.JobTemplateSpec, object runtime.Object, stepName string) error {
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
	job := &batch.Job{
		ObjectMeta: k8sApi.ObjectMeta{
			Labels:       desiredLabels,
			Annotations:  desiredAnnotations,
			GenerateName: prefix,
		},
	}

	if err := k8sApi.Scheme.Convert(&template.Spec, &job.Spec); err != nil {
		return fmt.Errorf("unable to convert job template: %v", err)
	}

	if newJob, err := w.KubeClient.Batch().Jobs(namespace).Create(job); err != nil {
		w.Recorder.Eventf(object, k8sApi.EventTypeWarning, "FailedCreate", "Error creating: %v", err)
		return fmt.Errorf("unable to create job: %v", err)
	} else {
		glog.V(4).Infof("Controller %v created job %v", meta.Name, newJob.Name)
	}
	return nil
}

func (w WorkflowJobControl) DeleteJob(namespace, jobName string, object runtime.Object) error {
	// @sdminonne: TODO once clientset is fixed implement DeleteJob
	/*
		accessor, err := meta.Accessor(object)
		if err != nil {
			return fmt.Errorf("object does not have ObjectMeta, %v", err)
		}
		if err := w.Client.Batch().Jobs(namespace).Delete(jobName, nil); err != nil {
			w.Recorder.Eventf(object, k8sApi.EventTypeWarning, "FailedDelete", "Error deleting: %v", err)
			return fmt.Errorf("unable to delete job: %v", err)
		} else {
			glog.V(4).Infof("Controller %v deleted job %v", accessor.GetName(), jobName)
			w.Recorder.Eventf(object, k8sApi.EventTypeNormal, "SuccessfulDelete", "Deleted job: %v", jobName)
		}
	*/
	return nil
}

type FakeJobControl struct {
	sync.Mutex
	CreatedJobTemplates []batch.JobTemplateSpec
	DeletedJobNames     []string
	Err                 error
}

var _ JobControlInterface = &FakeJobControl{}

func (f *FakeJobControl) CreateJob(namespace string, template *batch.JobTemplateSpec, object runtime.Object, key string) error {
	f.Lock()
	defer f.Unlock()
	f.CreatedJobTemplates = append(f.CreatedJobTemplates, *template)
	return nil
}

func (f *FakeJobControl) DeleteJob(namespace, name string, object runtime.Object) error {
	f.Lock()
	defer f.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.DeletedJobNames = append(f.DeletedJobNames, name)
	return nil
}
