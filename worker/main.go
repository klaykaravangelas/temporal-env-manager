package main

import (
	"log"

	ec2activities "github.com/klaykaravangelas/temporal-env-manager/activities/ec2"
	s3lambdaactivities "github.com/klaykaravangelas/temporal-env-manager/activities/s3lambda"
	ec2workflow "github.com/klaykaravangelas/temporal-env-manager/workflows/ec2"
	"github.com/klaykaravangelas/temporal-env-manager/workflows/router"
	s3lambdaworkflow "github.com/klaykaravangelas/temporal-env-manager/workflows/s3lambda"

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

	w.RegisterWorkflow(router.RouterWorkflow)
	w.RegisterWorkflow(ec2workflow.EC2EnvironmentWorkflow)
	w.RegisterActivity(ec2activities.EC2TerraformApply)
	w.RegisterActivity(ec2activities.EC2WaitForInstance)
	w.RegisterActivity(ec2activities.EC2RunSetupCommands)
	w.RegisterActivity(ec2activities.EC2TerraformDestroy)
	w.RegisterWorkflow(s3lambdaworkflow.S3LambdaEnvironmentWorkflow)
	w.RegisterActivity(s3lambdaactivities.S3LambdaTerraformApply)
	w.RegisterActivity(s3lambdaactivities.S3LambdaTerraformDestroy)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
