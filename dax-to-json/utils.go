package main

import (
	"fmt"
	"math/rand"
	"path"
	"strings"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
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
	millicores := rand.Intn(50) * 10
	step := api.WorkflowStep{
		Dependencies: dependencies,
		JobTemplate: &k8sBatch.JobTemplateSpec{
			ObjectMeta: k8sApi.ObjectMeta{
				Name: name,
			},
			Spec: k8sBatch.JobSpec{
				Parallelism: func(p int32) *int32 { return &p }(1),
				Template: k8sApi.PodTemplateSpec{
					ObjectMeta: k8sApi.ObjectMeta{
						Name: name + "-pod",
					},
					Spec: k8sApi.PodSpec{
						RestartPolicy: "OnFailure",
						Containers: []k8sApi.Container{
							{
								Image: "jess/stress",
								Name:  name + "-container",
								Args: []string{
									"-c", "1",
									"-t", "30",
								},
								Resources: k8sApi.ResourceRequirements{
									Limits: k8sApi.ResourceList{
										k8sApi.ResourceCPU: resource.MustParse(fmt.Sprintf("%vm", millicores)),
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
			"scheduler.alpha.kubernetes.io/name": "heat-scheduler",
		}
	}
	return step
}
