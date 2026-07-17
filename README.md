# Temporal Environment Manager

A Go project that uses [Temporal](https://temporal.io) to orchestrate the lifecycle of temporary AWS environments. Spin up an EC2 instance via Terraform, run setup commands via SSM, keep it alive for a configurable TTL, and automatically tear it down — all through a simple HTTP API.

## What it does

- Provisions a VPC, subnet, security group, IAM role, and EC2 instance via Terraform
- Waits for the instance to pass health checks
- Runs setup commands on the instance via AWS SSM (no SSH needed)
- Keeps the environment alive for a configurable TTL
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
    │  Executes activities
    ├── TerraformApply    → provisions AWS infrastructure
    ├── WaitForInstance   → polls EC2 until healthy
    ├── RunSetupCommands  → runs SSM commands on instance
    └── TerraformDestroy  → tears down all infrastructure
```

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Terraform](https://developer.hashicorp.com/terraform/install)
- [Docker](https://www.docker.com/products/docker-desktop/) and Docker Compose
- AWS account with credentials configured (`aws configure`)
- An S3 bucket for Terraform state storage (create one via the AWS console or CLI: `aws s3 mb s3://your-bucket-name`)

## Setup

### 1. Clone the repository

```bash
git clone https://github.com/klaykaravangelas/temporal-env-manager.git
cd temporal-env-manager
```

### 2. Configure Terraform state bucket

In `terraform/main.tf`, update the S3 backend with your bucket name:

```hcl
backend "s3" {
  bucket = "your-state-bucket-name"
  key    = "terraform.tfstate"
  region = "us-east-1"
}
```

### 3. Start the Temporal server

```bash
docker-compose up -d
```

This starts:
- PostgreSQL (Temporal's database)
- Temporal server on port `7233`
- Temporal UI on port `8080` → http://localhost:8080

### 4. Install Go dependencies

```bash
go mod download
```

### 5. Start the worker (Terminal 1)

```bash
go run worker/main.go
```

### 6. Start the API server (Terminal 2)

```bash
go run api/main.go
```

The API runs on `http://127.0.0.1:8090`.

## API Usage

### Spin up an environment

```bash
curl -X POST http://127.0.0.1:8090/environments
```

Response:
```json
{"workflowId": "env-1719432000000", "runId": "abc-123"}
```

### Check environment status

```bash
curl http://127.0.0.1:8090/environments/env-1719432000000
```

Response:
```json
{"step": "sleeping", "instanceId": "i-abc123"}
```

Possible steps: `initializing`, `applying`, `waiting-for-instance`, `running-setup`, `sleeping`, `destroying`, `completed`

### Extend the TTL

```bash
curl -X POST http://127.0.0.1:8090/environments/env-1719432000000/extend \
  -H "Content-Type: application/json" \
  -d '{"minutes": 10}'
```

### Tear down early

```bash
curl -X DELETE http://127.0.0.1:8090/environments/env-1719432000000
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
│   ├── terraform_apply.go       # Runs terraform init + apply, returns instance ID
│   ├── wait_for_instance.go     # Polls EC2 until instance is healthy
│   ├── run_setup_commands.go    # Runs SSM commands on the instance
│   └── terraform_destroy.go     # Runs terraform destroy
├── workflow/
│   ├── workflow.go              # Orchestrates activities, handles signals and queries
│   └── workflow_test.go         # In-memory workflow tests
├── worker/
│   └── main.go                  # Registers and runs the Temporal worker
├── api/
│   └── main.go                  # HTTP API server
├── terraform/
│   └── main.tf                  # AWS infrastructure definition
├── docker-compose.yml           # Temporal server + UI
└── go.mod
```
