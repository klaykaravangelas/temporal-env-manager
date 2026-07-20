# Temporal Environment Manager

A Go project that uses [Temporal](https://temporal.io) to orchestrate the lifecycle of temporary AWS environments. Provision infrastructure via Terraform, keep it alive for a configurable TTL, and automatically tear it down — all through a simple HTTP API.

## What it does

- Supports multiple environment types, each with its own Terraform config and workflow
- **EC2**: Provisions a VPC, subnet, security group, IAM role, and EC2 instance. Waits for health checks and runs setup commands via AWS SSM (no SSH needed)
- **S3Lambda**: Provisions an S3 bucket and Lambda function (Python 3.13) with S3 event notifications
- Keeps environments alive for a configurable TTL, or indefinitely until a teardown signal
- Automatically tears everything down when the TTL expires
- Supports extending the TTL or triggering early teardown via API signals

## Architecture

```
HTTP Client
    │
    ▼
API Server (localhost:8090)
    │  StartWorkflow / SignalWorkflow / QueryWorkflow
    ▼
Temporal Server (localhost:7233)
    │  Dispatches tasks via "environment-task-queue"
    ▼
Worker
    │  Executes workflows and activities
    │
    ├── RouterWorkflow
    │     └── Starts the correct child workflow based on request type
    │
    ├── EC2EnvironmentWorkflow
    │     ├── EC2TerraformApply       → provisions VPC, subnet, EC2 instance
    │     ├── EC2WaitForInstance      → polls EC2 until healthy
    │     ├── EC2RunSetupCommands     → runs SSM commands on instance
    │     └── EC2TerraformDestroy     → tears down all EC2 infrastructure
    │
    └── S3LambdaEnvironmentWorkflow
          ├── S3LambdaTerraformApply  → provisions S3 bucket + Lambda
          └── S3LambdaTerraformDestroy → tears down all S3Lambda infrastructure
```

### Router pattern

Every request goes through a `RouterWorkflow` which starts the appropriate child workflow and immediately completes. The child workflow then runs independently — if the router finishes or is cancelled, the child keeps running (`PARENT_CLOSE_POLICY_ABANDON`). All subsequent API calls (status, extend, teardown) go directly to the child workflow.

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Terraform](https://developer.hashicorp.com/terraform/install)
- [Docker](https://www.docker.com/products/docker-desktop/) and Docker Compose
- AWS account with credentials configured
- An S3 bucket for Terraform state storage (`aws s3 mb s3://your-bucket-name`)

## Setup

### 1. Clone the repository

```bash
git clone https://github.com/klaykaravangelas/temporal-env-manager.git
cd temporal-env-manager
```

### 2. Configure Terraform state bucket

In both `terraform/ec2/providers.tf` and `terraform/s3lambda/providers.tf`, update the S3 backend with your bucket name:

```hcl
backend "s3" {
  bucket = "your-state-bucket-name"
  key    = "ec2/terraform.tfstate"   # or s3lambda/terraform.tfstate
  region = "us-east-1"
}
```

### 3. Export AWS credentials

Terraform requires credentials to be available as environment variables. If you use AWS SSO, run this before starting the worker:

```bash
eval $(aws configure export-credentials --format env)
```

### 4. Start the Temporal server

```bash
docker-compose up -d
```

This starts:
- PostgreSQL (Temporal's database)
- Temporal server on port `7233`
- Temporal UI on port `8080` → http://localhost:8080

### 5. Install Go dependencies

```bash
go mod download
```

### 6. Start the worker (Terminal 1)

```bash
go run worker/main.go
```

### 7. Start the API server (Terminal 2)

```bash
go run api/main.go
```

The API runs on `http://127.0.0.1:8090`.

## API Usage

### Spin up an environment

```bash
curl -X POST http://127.0.0.1:8090/environments \
  -H "Content-Type: application/json" \
  -d '{"type": "ec2", "ttlMinutes": 30}'
```

```bash
curl -X POST http://127.0.0.1:8090/environments \
  -H "Content-Type: application/json" \
  -d '{"type": "s3lambda", "ttlMinutes": 10}'
```

Omit `ttlMinutes` to keep the environment alive until you explicitly delete it:

```bash
curl -X POST http://127.0.0.1:8090/environments \
  -H "Content-Type: application/json" \
  -d '{"type": "s3lambda"}'
```

Response:
```json
{"workflowId": "s3lambda-1719432000000"}
```

### Check environment status

```bash
curl http://127.0.0.1:8090/environments/s3lambda-1719432000000
```

EC2 response:
```json
{"step": "sleeping", "instanceId": "i-abc123"}
```

S3Lambda response:
```json
{"step": "sleeping", "bucketName": "temporal-poc-s3lambda-bucket"}
```

Possible steps: `initializing`, `applying`, `waiting-for-instance` (EC2 only), `running-setup` (EC2 only), `sleeping`, `destroying`, `completed`

### Extend the TTL

```bash
curl -X POST http://127.0.0.1:8090/environments/s3lambda-1719432000000/extend \
  -H "Content-Type: application/json" \
  -d '{"minutes": 10}'
```

### Tear down early

```bash
curl -X DELETE http://127.0.0.1:8090/environments/s3lambda-1719432000000
```

## Running Tests

No Temporal server or AWS credentials needed — tests run entirely in memory.

```bash
go test ./... -v
```

## Project Structure

```
temporal-env-manager/
├── activities/
│   ├── ec2/
│   │   ├── terraform_apply.go       # Runs terraform init + apply, returns instance ID and VPC ID
│   │   ├── wait_for_instance.go     # Polls EC2 until instance passes health checks
│   │   ├── run_setup_commands.go    # Runs SSM commands on the instance
│   │   └── terraform_destroy.go     # Runs terraform destroy
│   └── s3lambda/
│       ├── terraform_apply.go       # Runs terraform init + apply, returns bucket name and Lambda ARN
│       └── terraform_destroy.go     # Runs terraform destroy
├── workflows/
│   ├── router/
│   │   └── router.go                # Routes requests to the correct child workflow by type
│   ├── ec2/
│   │   ├── workflow.go              # EC2 workflow: apply → wait → setup → sleep → destroy
│   │   └── workflow_test.go         # In-memory EC2 workflow tests
│   └── s3lambda/
│       ├── workflow.go              # S3Lambda workflow: apply → sleep → destroy
│       └── workflow_test.go         # In-memory S3Lambda workflow tests
├── worker/
│   └── main.go                      # Registers all workflows and activities, runs the worker
├── api/
│   └── main.go                      # HTTP API server
├── terraform/
│   ├── ec2/
│   │   ├── providers.tf             # S3 backend, AWS provider
│   │   ├── variables.tf             # region, instance_type
│   │   ├── main.tf                  # VPC, subnet, IGW, security group, IAM role, EC2 instance
│   │   └── outputs.tf               # instance_id, vpc_id
│   └── s3lambda/
│       ├── providers.tf             # S3 backend, AWS provider
│       ├── variables.tf             # region, bucket_name, lambda_function_name
│       ├── main.tf                  # IAM role, Lambda, S3 bucket, event notification
│       ├── outputs.tf               # bucket_name, lambda_function_name, lambda_arn
│       └── lambda/
│           └── handler.py           # Python 3.13 Lambda that logs S3 event details
├── docker-compose.yml               # Temporal server + UI
└── go.mod
```
