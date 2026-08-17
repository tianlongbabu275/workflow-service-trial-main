WORKFLOW_SERVICE_NAME := workflow-service
WORKFLOW_GO_MAIN ?= github.com/sugerio/workflow-service-trial/cmd/workflow-service

GOCMD ?= $(shell which go)
GOTEST ?= $(GOCMD) test
GO_VERSION := $(shell $(GOCMD) version)
GO_OUTDIR ?= bin
GO_MAIN ?= cmd/marketplace/*.go
GO_USE_VENDOR ?= -mod=vendor

GO111MODULE := on
export GO111MODULE # Force go mod on.

.PHONY: build-go
build-go:
	$(GOCMD) build $(GO_USE_VENDOR) -o $(GO_OUTDIR)/$(WORKFLOW_SERVICE_NAME) $(WORKFLOW_GO_MAIN)

.PHONY: test-form-trigger
test-form-trigger:
	$(GOTEST) -v ./service/workflow_service/nodes/form_trigger

.PHONY: test-form-trigger-e2e
test-form-trigger-e2e:
	$(GOTEST) -v service/workflow_service/nodes_test/init_test.go service/workflow_service/nodes_test/form_trigger_test.go

.PHONY: preview-form-trigger
preview-form-trigger:
	$(GOTEST) -v ./service/workflow_service/nodes/form_trigger -run TestRenderHTML_WritesPreviewFile
	@echo "Open service/workflow_service/nodes/form_trigger/testdata/contact-us.preview.html in a browser"

.PHONY: clean-go
clean-go:
	rm -rf $(MARKETPLACE_SERVICE_NAME) $(GO_OUTDIR)
	rm -rf $(PARTNER_SERVICE_NAME) $(GO_OUTDIR)

.PHONY: show-go
show-go:
	@echo "GOCMD: $(GOCMD)"
	@echo "GO_VERSION: $(GO_VERSION)"
	@echo "GO_OUTDIR: $(GO_OUTDIR)"
	@echo "GO_MAIN: $(GO_MAIN)"
