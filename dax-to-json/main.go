package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strings"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
	k8sApi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	k8sApiUnv "k8s.io/kubernetes/pkg/api/unversioned"
	k8sBatch "k8s.io/kubernetes/pkg/apis/batch"
)

// Adag is the root element of the dax file.
type Adag struct {
	XMLName  xml.Name `xml:"adag"`
	Jobs     []Job    `xml:"job"`
	Childs   []Child  `xml:"child"`
	ChildMap map[string][]string
}

// Job is a step in the DAG.
type Job struct {
	XMLName xml.Name `xml:"job"`
	ID      string   `xml:"id,attr"`
}

// Child is used to describe dependencies. A child has parents, which are its
// dependencies. Child.Ref maps to Job.ID.
type Child struct {
	XMLName xml.Name `xml:"child"`
	Ref     string   `xml:"ref,attr"`
	Parents []Parent `xml:"parent"`
}

// Parents is a dependency of a child.
type Parent struct {
	XMLName xml.Name `xml:"parent"`
	Ref     string   `xml:"ref,attr"`
}

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

// main converts a dax file to a json file.
func main() {
	scheduling := flag.Bool("scheduling", true, "Use heat scheduling.")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Printf("Expecting file as first parameter. USAGE: dax-to-yaml [file] [wf-name].")
		return
	}
	name := "test-workflow"
	if flag.NArg() == 2 {
		name = flag.Arg(1)
	}

	// Read specified dax file.
	file, err := os.Open(flag.Arg(0)) // For read access.
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

	// Parse dax file.
	adag := Adag{}
	err = xml.Unmarshal(data, &adag)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	// Map jobs to dependencies.
	adag.ChildMap = make(map[string][]string)
	for _, child := range adag.Childs {
		var dependencies []string
		for _, dep := range child.Parents {
			dependencies = append(dependencies, strings.ToLower(dep.Ref))
		}
		adag.ChildMap[child.Ref] = dependencies
	}

	// Create the workflow.
	wf := newWorkflow(name)
	for _, job := range adag.Jobs {
		wf.Spec.Steps[strings.ToLower(job.ID)] = newStep(job.ID, adag.ChildMap[job.ID], *scheduling)
	}

	// Parse the workflow and print it to std out.
	b, err := json.Marshal(wf)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
}
