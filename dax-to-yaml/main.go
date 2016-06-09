package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
)

type Adag struct {
	XMLName  xml.Name `xml:"adag"`
	Jobs     []Job    `xml:"job"`
	Childs   []Child  `xml:"child"`
	ChildMap map[string][]string
}

type Job struct {
	XMLName xml.Name `xml:"job"`
	ID      string   `xml:"id,attr"`
}

type Child struct {
	XMLName xml.Name `xml:"child"`
	Ref     string   `xml:"ref,attr"`
	Parents []Parent `xml:"parent"`
}

type Parent struct {
	XMLName xml.Name `xml:"parent"`
	Ref     string   `xml:"ref,attr"`
}

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

func newStep(name string, dependencies []string) api.WorkflowStep {
	return api.WorkflowStep{
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
						Annotations: map[string]string{
							"scheduler.alpha.kubernetes.io/name": "heat-scheduler",
						},
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
							},
						},
					},
				},
			},
		},
	}
}

func main() {
	if os.Args[1] == "" {
		fmt.Printf("Expecting file as first parameter. USAGE: dax-to-yaml [file] [wf-name].")
		return
	}
	name := ""
	if len(os.Args) == 2 {
		name = "test-workflow"
	}
	file, err := os.Open(os.Args[1]) // For read access.
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	adag := Adag{}
	err = xml.Unmarshal(data, &adag)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	// Map jobs to dependencies
	adag.ChildMap = make(map[string][]string)
	for _, child := range adag.Childs {
		var dependencies []string
		for _, dep := range child.Parents {
			dependencies = append(dependencies, dep.Ref)
		}
		adag.ChildMap[child.Ref] = dependencies
	}

	wf := newWorkflow(name)
	for _, job := range adag.Jobs {
		wf.Spec.Steps[job.ID] = newStep(job.ID, adag.ChildMap[job.ID])
	}

	b, err := json.Marshal(wf)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
}
