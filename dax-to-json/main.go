package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/nov1n/kubernetes-workflow/pkg/api"
)

var (
	// scheduling specifies whether to use the heat scheduling annotation or not.
	scheduling = flag.Bool("scheduling", true, "Use heat scheduling.")
	// defaultWorkflowName is the default workflow name.
	defaultWorkflowName = "test-workflow"
)

// getWorkflowName gets a workflow name that was specified as a cli argument.
func getWorkflowName() string {
	name := defaultWorkflowName
	if flag.NArg() == 2 {
		name = flag.Arg(1)
	}
	return name
}

// parseDax parses a dax file to an Adag.
func parseDax(filename string) (*Adag, error) {
	// Read specified dax file.
	file, err := os.Open(filename) // For read access.
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Parse dax file.
	adag := &Adag{}
	err = xml.Unmarshal(data, adag)
	if err != nil {
		return nil, err
	}
	return adag, nil
}

// mapJobsToDependencies maps childs and parents to jobs and dependencies.
func mapJobsToDependencies(adag *Adag) {
	adag.ChildMap = make(map[string][]string)
	for _, child := range adag.Childs {
		var dependencies []string
		for _, dep := range child.Parents {
			dependencies = append(dependencies, strings.ToLower(dep.Ref))
		}
		adag.ChildMap[child.Ref] = dependencies
	}

}

// printWorkflow prints a workflow in JSON.
func printWorkflow(wf *api.Workflow) {
	b, err := json.Marshal(wf)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
}

// main converts a dax file to a json file.
func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Printf("Expecting file as first parameter. USAGE: dax-to-yaml [file] [wf-name].")
		return
	}

	adag, err := parseDax(flag.Arg(0))
	if err != nil {
		fmt.Printf("Error when parsing dax to adag: %v\n", err)
		return
	}

	mapJobsToDependencies(adag)

	// Create the workflow.
	wf := newWorkflow(getWorkflowName())
	for _, job := range adag.Jobs {
		wf.Spec.Steps[strings.ToLower(job.ID)] = newStep(job.ID, adag.ChildMap[job.ID], *scheduling)
	}
	printWorkflow(wf)
}
