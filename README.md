
# Install Docker Desktop
https://docs.docker.com/desktop/install/mac-install/

Start the docker app after the installment.

## Install go@1.23
```sh
   brew install go@1.23
   echo 'export PATH="/opt/homebrew/opt/go@1.23/bin:$PATH"' >> ~/.zshrc
```

# Run Local Unit Tests
```bash
# start docker compose
docker compose up -d

# run test on webhook node
go test -v service/workflow_service/nodes_test/init_test.go  service/workflow_service/nodes_test/webhook_test.go

# run test on if node
go test -v service/workflow_service/nodes_test/init_test.go service/workflow_service/nodes_test/if_test.go

# run test on code node
go test -v service/workflow_service/nodes_test/init_test.go service/workflow_service/nodes_test/code_test.go

# run test on filter node
go test -v service/workflow_service/nodes_test/init_test.go service/workflow_service/nodes_test/filter_test.go

# run test on switch node
go test -v service/workflow_service/nodes_test/init_test.go service/workflow_service/nodes_test/switch_test.go

# run test on form trigger node (HTML GET + form POST)
go test -v service/workflow_service/nodes_test/init_test.go service/workflow_service/nodes_test/form_trigger_test.go

# Shut down the docker compose
docker compose down
```

## Demo Form Trigger

This service does not listen on a public HTTP port in test mode, so you cannot `curl localhost` against a running API. Use the two checks below.

**1. Look at the form (no Docker)**

```bash
make preview-form-trigger
open service/workflow_service/nodes/form_trigger/testdata/contact-us.preview.html
```

You should see title `Contact Us Now Customize`, fields Name / Email / Address / Hobby, and a Submit button.

**2. Prove GET + POST run the workflow (needs Docker)**

```bash
docker compose up -d
make test-form-trigger          # parse + HTML unit tests
make test-form-trigger-e2e      # create workflow, GET html, POST submit, Code node adds myNewField
```

Success looks like `PASS` for:

- `TestFormTrigger_Call_Active_Workflow_GET_and_POST` — production URL
- `TestFormTrigger_Test_Webhook_GET_and_POST` — test URL (`isTest=true`)
- `TestFormTrigger_Inactive_Workflow_Rejects_GET` — form is 404 until the workflow is active

# FYI
Common developing commands
```sh
   go mod tidy
   go mod vendor
   make build-go
   make clean-go
```

