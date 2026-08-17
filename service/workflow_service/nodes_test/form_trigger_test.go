package nodes_test

// Command to run this test only
// go test -v service/workflow_service/nodes_test/init_test.go service/workflow_service/nodes_test/form_trigger_test.go

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/sugerio/workflow-service-trial/service/workflow_service/api"
	"github.com/sugerio/workflow-service-trial/service/workflow_service/core"
	formtrigger "github.com/sugerio/workflow-service-trial/service/workflow_service/nodes/form_trigger"
	"github.com/sugerio/workflow-service-trial/shared/structs"
	"github.com/valyala/fasthttp"
)

type FormTriggerTestSuite struct {
	suite.Suite
}

func Test_FormTrigger(t *testing.T) {
	suite.Run(t, new(FormTriggerTestSuite))
}

func (s *FormTriggerTestSuite) Test() {
	s.T().Run("TestFormTriggerSpec", func(t *testing.T) {
		t.Parallel()
		assert := require.New(s.T())

		var nodeSpec structs.WorkflowNodeDescriptionSpec
		testFile, err := os.ReadFile("../nodes/form_trigger/node.json")
		assert.Nil(err)
		err = json.Unmarshal(testFile, &nodeSpec)
		assert.Nil(err)
		assert.Equal("Waiting for you to submit the form", nodeSpec.EventTriggerDescription)
		assert.Equal("n8n Form Trigger", nodeSpec.Defaults.Name)
		assert.Equal(formtrigger.Name, nodeSpec.Name)
	})

	s.T().Run("TestFormTriggerGenerate", func(t *testing.T) {
		t.Parallel()
		assert := require.New(s.T())

		executor := core.NewExecutor(formtrigger.Name)
		node := executor.GetNode()
		assert.NotNil(node)
		spec := node.DefaultSpec().(*structs.WorkflowNodeSpec).NodeSpec
		assert.Equal("n8n Form Trigger", spec.DisplayName)
		assert.Equal("/icons/embed/n8n-nodes-base.formTrigger/form.svg", spec.IconUrl)
	})

	s.T().Run("TestFormTriggerExecute", func(t *testing.T) {
		t.Parallel()
		assert := require.New(s.T())

		rawData, err := os.ReadFile("./test_files/form-trigger-e2e.json")
		assert.Nil(err)
		var workflow structs.WorkflowEntity
		assert.Nil(json.Unmarshal(rawData, &workflow))
		formNode := workflow.Nodes[0]

		req := fasthttpRequestFromForm(url.Values{
			"Name":    []string{"Ada"},
			"Email":   []string{"ada@example.com"},
			"Address": []string{"1 Market St"},
			"Hobby":   []string{"basketball"},
		}, false)

		result := (&formtrigger.FormTrigger{}).Execute(context.Background(), &structs.NodeExecuteInput{
			Params: &formNode,
			AdditionalData: &structs.WorkflowExecuteAdditionalData{
				HttpRequest: req,
			},
		})
		assert.Equal(structs.WorkflowExecutionStatus_Success, result.ExecutionStatus)
		jsonField := result.TriggerData[0]["json"].(map[string]interface{})
		assert.Equal("Ada", jsonField["Name"])
		assert.Equal("basketball", jsonField["Hobby"])
		assert.Equal("production", jsonField["formMode"])
	})

	s.T().Run("TestFormTrigger Call Active Workflow GET and POST", func(t *testing.T) {
		t.Parallel()
		assert := require.New(s.T())

		organization := structs.CreateOrganization_Testing(rdsDbQueries, sid, "")
		newWorkflow, err := api.CreateWorkflow_Testing(
			testFiberLambda, organization.ID, "./test_files/form-trigger-e2e.json")
		assert.Nil(err)
		workflowID := newWorkflow.ID

		err = api.ActivateWorkflow_Testing(testFiberLambda, organization.ID, workflowID)
		assert.Nil(err)

		nodeId, webhookId, err := api.GetFormTriggerIdAndNodeIdInWorkflow(newWorkflow)
		assert.Nil(err)

		entities, err := api.GetWebhookEntities(workflowID, webhookId)
		assert.Nil(err)
		assert.Equal(1, len(entities))
		assert.Equal(http.MethodPost, entities[0].Method)

		getResponse, err := api.CallWebhookRaw_Testing(
			testFiberLambda, http.MethodGet, workflowID, nodeId, webhookId, false, "", "text/html")
		assert.Nil(err)
		assert.Equal(200, getResponse.StatusCode)
		assert.Contains(headerValue(getResponse.Headers, getResponse.MultiValueHeaders, "Content-Type"), "text/html")
		assert.Contains(getResponse.Body, "Contact Us Now Customize")
		assert.Contains(getResponse.Body, "get back to you soon")
		assert.Contains(getResponse.Body, `name="Name"`)
		assert.Contains(getResponse.Body, `type="email"`)
		assert.Contains(getResponse.Body, "basketball")
		assert.Contains(getResponse.Body, "soccer")
		assert.Contains(getResponse.Body, `method="post"`)
		t.Logf("GET form HTML length=%d workflowId=%s nodeId=%s webhookId=%s",
			len(getResponse.Body), workflowID, nodeId, webhookId)

		formBody := url.Values{
			"Name":    []string{"Ada"},
			"Email":   []string{"ada@example.com"},
			"Address": []string{"1 Market St"},
			"Hobby":   []string{"basketball"},
		}.Encode()
		postResponse, err := api.CallWebhookRaw_Testing(
			testFiberLambda, http.MethodPost, workflowID, nodeId, webhookId, false,
			formBody, "application/x-www-form-urlencoded")
		assert.Nil(err)
		assert.Equal(200, postResponse.StatusCode)

		var postBody map[string]interface{}
		assert.Nil(json.Unmarshal([]byte(postResponse.Body), &postBody))
		assert.Equal("Workflow was started", postBody["message"])
		executionId, _ := postBody["executionId"].(string)
		assert.NotEmpty(executionId)

		execution, err := api.WaitWorkflowExecution_Testing(
			testFiberLambda, organization.ID, executionId, 10*time.Second)
		assert.Nil(err)
		assert.Equal(structs.WorkflowExecutionStatus_Success, execution.Status)
		codeTask := execution.Data.ResultData.RunData["Code"][0]
		codeJSON := codeTask.Data["main"][0][0]["json"].(map[string]interface{})
		assert.Equal("Ada", codeJSON["Name"])
		assert.Equal("ada@example.com", codeJSON["Email"])
		assert.Equal("1 Market St", codeJSON["Address"])
		assert.Equal("basketball", codeJSON["Hobby"])
		assert.Equal(float64(1), codeJSON["myNewField"])

		formTriggerTask := execution.Data.ResultData.RunData["n8n Form Trigger"][0]
		formJSON := formTriggerTask.Data["main"][0][0]["json"].(map[string]interface{})
		assert.Equal("production", formJSON["formMode"])

		err = api.DeactivateWorkflow_Testing(testFiberLambda, organization.ID, workflowID)
		assert.Nil(err)
		entities, err = api.GetWebhookEntities(workflowID, webhookId)
		assert.Nil(err)
		assert.Equal(0, len(entities))

		getAfter, err := api.CallWebhookRaw_Testing(
			testFiberLambda, http.MethodGet, workflowID, nodeId, webhookId, false, "", "text/html")
		assert.Nil(err)
		assert.NotEqual(200, getAfter.StatusCode)

		err = api.DeleteWorkflow_Testing(testFiberLambda, organization.ID, workflowID)
		assert.Nil(err)
	})

	s.T().Run("TestFormTrigger Inactive Workflow Rejects GET", func(t *testing.T) {
		t.Parallel()
		assert := require.New(s.T())

		organization := structs.CreateOrganization_Testing(rdsDbQueries, sid, "")
		newWorkflow, err := api.CreateWorkflow_Testing(
			testFiberLambda, organization.ID, "./test_files/form-trigger-e2e.json")
		assert.Nil(err)
		nodeId, webhookId, err := api.GetFormTriggerIdAndNodeIdInWorkflow(newWorkflow)
		assert.Nil(err)

		getResponse, err := api.CallWebhookRaw_Testing(
			testFiberLambda, http.MethodGet, newWorkflow.ID, nodeId, webhookId, false, "", "text/html")
		assert.Nil(err)
		assert.NotEqual(200, getResponse.StatusCode)

		err = api.DeleteWorkflow_Testing(testFiberLambda, organization.ID, newWorkflow.ID)
		assert.Nil(err)
	})

	s.T().Run("TestFormTrigger Test Webhook GET and POST", func(t *testing.T) {
		t.Parallel()
		assert := require.New(s.T())

		organization := structs.CreateOrganization_Testing(rdsDbQueries, sid, "")
		newWorkflow, err := api.CreateWorkflow_Testing(
			testFiberLambda, organization.ID, "./test_files/form-trigger-e2e.json")
		assert.Nil(err)
		workflowID := newWorkflow.ID

		workflowRunResponse, err := api.ManualRunWorkflowFullResponse_Testing(testFiberLambda, newWorkflow)
		assert.Nil(err)
		assert.True(workflowRunResponse.Data.WaitingForWebhook)

		nodeId, webhookId, err := api.GetFormTriggerIdAndNodeIdInWorkflow(newWorkflow)
		assert.Nil(err)

		getResponse, err := api.CallWebhookRaw_Testing(
			testFiberLambda, http.MethodGet, workflowID, nodeId, webhookId, true, "", "text/html")
		assert.Nil(err)
		assert.Equal(200, getResponse.StatusCode)
		assert.Contains(getResponse.Body, "Contact Us Now Customize")
		assert.Contains(getResponse.Body, "isTest=true")

		formBody := url.Values{
			"Name":    []string{"Grace"},
			"Email":   []string{"grace@example.com"},
			"Address": []string{"2 Market St"},
			"Hobby":   []string{"soccer"},
		}.Encode()
		postResponse, err := api.CallWebhookRaw_Testing(
			testFiberLambda, http.MethodPost, workflowID, nodeId, webhookId, true,
			formBody, "application/x-www-form-urlencoded")
		assert.Nil(err)
		assert.Equal(200, postResponse.StatusCode)
		var postBody map[string]interface{}
		assert.Nil(json.Unmarshal([]byte(postResponse.Body), &postBody))
		executionId, _ := postBody["executionId"].(string)
		assert.NotEmpty(executionId)

		execution, err := api.WaitWorkflowExecution_Testing(
			testFiberLambda, organization.ID, executionId, 10*time.Second)
		assert.Nil(err)
		assert.Equal(structs.WorkflowExecutionStatus_Success, execution.Status)
		codeJSON := execution.Data.ResultData.RunData["Code"][0].Data["main"][0][0]["json"].(map[string]interface{})
		assert.Equal("Grace", codeJSON["Name"])
		assert.Equal("soccer", codeJSON["Hobby"])
		assert.Equal(float64(1), codeJSON["myNewField"])
		formJSON := execution.Data.ResultData.RunData["n8n Form Trigger"][0].Data["main"][0][0]["json"].(map[string]interface{})
		assert.Equal("test", formJSON["formMode"])

		deleted, err := api.DeleteTestWebhook_Testing(testFiberLambda, organization.ID, workflowID)
		assert.Nil(err)
		assert.True(deleted)

		getAfter, err := api.CallWebhookRaw_Testing(
			testFiberLambda, http.MethodGet, workflowID, nodeId, webhookId, true, "", "text/html")
		assert.Nil(err)
		assert.NotEqual(200, getAfter.StatusCode)

		err = api.DeleteWorkflow_Testing(testFiberLambda, organization.ID, workflowID)
		assert.Nil(err)
	})
}

func fasthttpRequestFromForm(values url.Values, isTest bool) *fasthttp.Request {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetRequestURI("/workflow/public/webhook/workflow/wf/node/n?webhookId=abc&isTest=" + strconv.FormatBool(isTest))
	req.SetBodyString(values.Encode())
	return req
}

func headerValue(headers map[string]string, multi map[string][]string, key string) string {
	if headers != nil {
		if value := headers[key]; value != "" {
			return value
		}
		if value := headers["content-type"]; key == "Content-Type" && value != "" {
			return value
		}
	}
	if multi != nil {
		if values := multi[key]; len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
