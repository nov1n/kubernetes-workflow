package main

import (
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"strings"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
)

const (
	// millicoresRange is used to generate a random number from 0 to 50, which
	// specifies the millicore limit for a pod.
	millicoresRange = 50
	// millicoreFactor is the factor used to multiple the outcome of the random
	// number generated by using millicoresRange.
	millicoreFactor = 10
	// noOfCores is the number of cores used to stress the cpu of a pod.
	noOfCores = "1"
	// timeout is the time after which the stress program and with it the pod time out.
	timeout = "30"
	// schedulingAnnotation is the annotation used to specify the scheduler for a pod.
	schedulingAnnotation = "scheduler.alpha.kubernetes.io/name"
	// heatSchedulerName is the name of the custom heat scheduler.
	heatSchedulerName = "heat-scheduler"
	// parallelism is the number of parallel pods in a job.
	parallelism = 1
)

// newWorkflow returns a new workflow with a given namen.
func newWorkflow(name string) *api.Workflow {
	return &api.Workflow{
		TypeMeta: k8sApiUnv.TypeMeta{
			APIVersion: path.Join(api.Group, api.Version),
			Kind:       api.Kind,
		},
		ObjectMeta: k8sApi.ObjectMeta{
			Name: name,
		},
		Spec: api.WorkflowSpec{
			ActiveDeadlineSeconds: func(a int64) *int64 { return &a }(3600),
			Steps: make(map[string]api.WorkflowStep),
		},
	}
}

// newStep returns a new step for a workflow given a name and a list of
// dependencies. A step gets a random amount of millicores assigned between
// 0 and 500.
func newStep(name string, dependencies []string, scheduling bool) api.WorkflowStep {
	name = strings.ToLower(name)
	load := rand.Intn(millicoresRange)
	step := api.WorkflowStep{
		Dependencies: dependencies,
		JobTemplate: &k8sBatch.JobTemplateSpec{
			ObjectMeta: k8sApi.ObjectMeta{
				Name: name,
			},
			Spec: k8sBatch.JobSpec{
				Parallelism: func(p int32) *int32 { return &p }(parallelism),
				Template: k8sApi.PodTemplateSpec{
					ObjectMeta: k8sApi.ObjectMeta{
						Name: name + "-pod",
					},
					Spec: k8sApi.PodSpec{
						RestartPolicy: "OnFailure",
						Containers: []k8sApi.Container{
							{
								Image: "lorel/docker-stress-ng",
								Name:  name + "-container",
								Args: []string{
									"--cpu", noOfCores,
									"--cpu-load", strconv.Itoa(load),
									"-t", timeout,
								},
								Resources: k8sApi.ResourceRequirements{
									Limits: k8sApi.ResourceList{
										k8sApi.ResourceCPU: resource.MustParse(fmt.Sprintf("%vm", load*millicoreFactor)),
									},
									Requests: k8sApi.ResourceList{
										k8sApi.ResourceCPU: resource.MustParse(fmt.Sprintf("%vm", load*millicoreFactor)),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if scheduling {
		step.JobTemplate.Spec.Template.ObjectMeta.Annotations = map[string]string{
			schedulingAnnotation: heatSchedulerName,
		}
	}
	return step
}