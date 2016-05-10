package job

import (
	"fmt"
	"testing"

	"github.com/nov1n/kubernetes-workflow/pkg/client"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
)

const (
	NAMESPACE = "my-workflows"
)

func TestJobCreation(t *testing.T) {
	// Create rest client
	client, err := client.NewRESTClient("127.0.0.1", "8080")
	if err != nil {
		t.Error("Could not create REST client.")
	}

	// Create namespace
	namespace := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: NAMESPACE,
		},
	}
	_, err = client.Namespaces().Create(namespace)
	if err != nil {
		t.Error(fmt.Sprintf("Could not create namespace '%s': %s", NAMESPACE, err))
	}

	// Add job
	jobManager := &Manager{
		Client:    client,
		Namespace: NAMESPACE,
	}

	jobConfig := &batch.Job{
		ObjectMeta: api.ObjectMeta{
			Name: "my-job",
		},
		Spec: batch.JobSpec{
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Name: "my-job",
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						api.Container{
							Name:  "nginx",
							Image: "nginx",
							Ports: []api.ContainerPort{
								api.ContainerPort{
									ContainerPort: 80,
								},
							},
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	_, err = jobManager.AddJob(jobConfig)

	if err != nil {
		t.Error(fmt.Sprintf("Could not add job: %v", err))
	}

	// Remove job
	err = jobManager.RemoveJob(jobConfig)
	if err != nil {
		t.Error(fmt.Sprintf("Could not delete job: %v", err))
	}

	// Cleanup
	err = client.Namespaces().Delete(NAMESPACE)
	if err != nil {
		t.Error(fmt.Sprintf("Could not delete namespace '%s': %s", NAMESPACE, err))
	}
}
