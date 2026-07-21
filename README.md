# Temporal Environment Manager

A Go project that uses [Temporal](https://temporal.io) to orchestrate the lifecycle of temporary AWS environments. Provision infrastructure via Terraform, keep it alive for a configurable TTL, and automatically tear it down — all through a simple HTTP API and a React UI.

## What it does

- Supports multiple environment types, each with its own Terraform config and workflow
- **EC2**: Provisions a VPC, subnet, security group, IAM role, and EC2 instance. Waits for health checks and runs setup commands via AWS SSM (no SSH needed)
- **S3Lambda**: Provisions an S3 bucket and Lambda function (Python 3.13) with S3 event notifications
- Keeps environments alive for a configurable TTL, or indefinitely until a teardown signal
- Automatically tears everything down when the TTL expires
- Supports extending the TTL or triggering early teardown via API signals

## Architecture

```
Browser (localhost:5173)
    │
    ▼
React UI (Vite dev server)
    │  GET/POST/DELETE /environments
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
- [Node.js 18+](https://nodejs.org/) (for the UI)
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

### 6. Install UI dependencies

```bash
cd ui && npm install && cd ..
```

### 7. Start everything

The easiest way is to use the startup script, which exports credentials, ensures Temporal is running, and opens the worker, API, and UI in separate terminal tabs:

```bash
./scripts/start.sh
```

Or start each process manually:

```bash
# Terminal 1
go run worker/main.go

# Terminal 2
go run api/main.go

# Terminal 3
cd ui && npm run dev
```

| Service | URL |
|---|---|
| UI | http://localhost:5173 |
| API | http://127.0.0.1:8090 |
| Temporal UI | http://localhost:8080 |

### Stopping

```bash
./scripts/stop.sh
```

This kills the worker, API, and UI processes — including the compiled child binaries that `go run` spawns. Without this, restarting the worker with `go run` leaves stale processes running in the background.

To also stop the Temporal Docker containers:

```bash
./scripts/stop.sh --temporal
```

> **Note:** `go run` compiles your code into a temporary binary and runs it as a child process. Closing the terminal or killing the `go run` parent does **not** kill the child. Always use `stop.sh` to ensure a clean shutdown.

## UI

Open `http://localhost:5173` in your browser. The UI lets you:

- Create EC2 or S3Lambda environments with a type selector and optional TTL
- Pass optional Terraform variable overrides per environment type (region, instance type, bucket name, etc.)
- View all running environments with their current step and a labeled output grid (Instance ID, VPC ID, Bucket Name, Lambda Function, Lambda ARN, Started At)
- Extend the TTL of a running environment
- Destroy an environment — the Destroy button is disabled until provisioning completes

The environment list polls every 5 seconds automatically.

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

Pass optional `vars` to override Terraform variable defaults:

```bash
# EC2 — override region and instance type
curl -X POST http://127.0.0.1:8090/environments \
  -H "Content-Type: application/json" \
  -d '{"type": "ec2", "ttlMinutes": 30, "vars": {"region": "us-west-2", "instanceType": "t3.small"}}'

# S3Lambda — override bucket and function name
curl -X POST http://127.0.0.1:8090/environments \
  -H "Content-Type: application/json" \
  -d '{"type": "s3lambda", "vars": {"bucketName": "my-bucket", "lambdaFunctionName": "my-handler"}}'
```

Any omitted `vars` keys fall back to the defaults defined in `terraform/*/variables.tf`.

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
{"step": "sleeping", "instanceId": "i-abc123", "vpcId": "vpc-0abc123"}
```

S3Lambda response:
```json
{"step": "sleeping", "bucketName": "my-bucket", "lambdaFunctionName": "my-handler", "lambdaArn": "arn:aws:lambda:us-east-1:123456789:function:my-handler"}
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

## CI

A GitHub Actions workflow runs on every push and pull request to `main`. It has two parallel jobs:

- **build-and-test**: compiles all Go packages and runs the full test suite
- **ui-build**: installs UI dependencies and runs `vite build`

No AWS credentials or Temporal server are needed — Go tests use the Temporal test SDK in-memory, and the UI build is purely static.

## Project Structure

```
temporal-env-manager/
├── .github/
│   └── workflows/
│       └── ci.yml                   # Build + test on push/PR to main
├── scripts/
│   ├── start.sh                     # Exports credentials, starts Temporal, opens worker/API/UI
│   └── stop.sh                      # Kills worker, API, UI processes (and optionally Temporal)
├── activities/
│   ├── ec2/
│   │   ├── terraform_apply.go       # Runs terraform init + apply, returns instance ID and VPC ID
│   │   ├── wait_for_instance.go     # Polls EC2 until instance passes health checks
│   │   ├── run_setup_commands.go    # Runs SSM commands on the instance
│   │   └── terraform_destroy.go     # Runs terraform destroy
│   └── s3lambda/
│       ├── terraform_apply.go       # Runs terraform init + apply, returns bucket name, Lambda function name, and ARN
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
│   └── main.go                      # HTTP API server with CORS middleware
├── ui/
│   ├── src/
│   │   ├── App.jsx                  # React UI: environment list, create form, extend/destroy actions
│   │   └── index.css                # Tailwind CSS import
│   ├── index.html
│   ├── vite.config.js               # Vite + Tailwind plugin
│   └── package.json
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
├── go.mod
└── README.md
```
