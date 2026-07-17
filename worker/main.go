package main

import (
	"log"

	"github.com/klaykaravangelas/temporal-env-manager/activities"
	envworkflow "github.com/klaykaravangelas/temporal-env-manager/workflow"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}
	defer c.Close()

	w := worker.New(c, "environment-task-queue", worker.Options{})

	w.RegisterWorkflow(envworkflow.EnvironmentWorkflow)
	w.RegisterActivity(activities.TerraformApply)
	w.RegisterActivity(activities.WaitForInstance)
	w.RegisterActivity(activities.RunSetupCommands)
	w.RegisterActivity(activities.TerraformDestroy)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
