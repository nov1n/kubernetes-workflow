package main

import (
	"fmt"
	"os"

	"github.com/nov1n/kubernetes-workflow/client"
	"github.com/nov1n/kubernetes-workflow/job"
)

func main() {
	client, err := client.NewRESTClient("127.0.0.1", "8080")

	if err != nil {
		panic(err)
	}

	jobManager := job.Manager{
		Client:    client,
		Namespace: "my-workflows",
	}

	fmt.Print("Created namespace", jobManager.Namespace)
	os.Exit(0)
}
