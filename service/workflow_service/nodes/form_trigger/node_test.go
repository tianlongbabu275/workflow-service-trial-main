package form_trigger

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/sugerio/workflow-service-trial/shared/structs"
	"github.com/valyala/fasthttp"
)

func newFormRequest(isTest bool, values url.Values) *fasthttp.Request {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetRequestURI("/workflow/public/webhook/workflow/wf-1/node/node-1?webhookId=abc&isTest=" + boolQuery(isTest))
	req.SetBodyString(values.Encode())
	return req
}

func boolQuery(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func TestExecute_ParsesFormFields(t *testing.T) {
	assert := require.New(t)
	node := sampleFormTriggerNode()
	req := newFormRequest(false, url.Values{
		"Name":    []string{"Ada"},
		"Email":   []string{"  ada@example.com  "},
		"Address": []string{"1 Market St"},
		"Hobby":   []string{"basketball"},
	})
	defer fasthttp.ReleaseRequest(req)

	result := (&FormTrigger{}).Execute(context.Background(), &structs.NodeExecuteInput{
		Params: node,
		AdditionalData: &structs.WorkflowExecuteAdditionalData{
			HttpRequest: req,
		},
	})

	assert.Equal(structs.WorkflowExecutionStatus_Success, result.ExecutionStatus)
	assert.Len(result.TriggerData, 1)
	jsonField, ok := result.TriggerData[0]["json"].(map[string]interface{})
	assert.True(ok)
	assert.Equal("Ada", jsonField["Name"])
	assert.Equal("ada@example.com", jsonField["Email"])
	assert.Equal("1 Market St", jsonField["Address"])
	assert.Equal("basketball", jsonField["Hobby"])
	assert.Equal("production", jsonField["formMode"])
	assert.NotEmpty(jsonField["submittedAt"])
}

func TestExecute_TestModeFromQuery(t *testing.T) {
	assert := require.New(t)
	req := newFormRequest(true, url.Values{
		"Name":    []string{"Ada"},
		"Email":   []string{"ada@example.com"},
		"Address": []string{"1 Market St"},
		"Hobby":   []string{"soccer"},
	})
	defer fasthttp.ReleaseRequest(req)

	result := (&FormTrigger{}).Execute(context.Background(), &structs.NodeExecuteInput{
		Params: sampleFormTriggerNode(),
		AdditionalData: &structs.WorkflowExecuteAdditionalData{
			HttpRequest: req,
		},
	})
	assert.Equal(structs.WorkflowExecutionStatus_Success, result.ExecutionStatus)
	jsonField := result.TriggerData[0]["json"].(map[string]interface{})
	assert.Equal("test", jsonField["formMode"])
}

func TestExecute_JSONBody(t *testing.T) {
	assert := require.New(t)
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetRequestURI("/form")
	body, _ := json.Marshal(map[string]string{
		"Name":    "Ada",
		"Email":   "ada@example.com",
		"Address": "1 Market St",
		"Hobby":   "basketball",
	})
	req.SetBody(body)

	result := (&FormTrigger{}).Execute(context.Background(), &structs.NodeExecuteInput{
		Params: sampleFormTriggerNode(),
		AdditionalData: &structs.WorkflowExecuteAdditionalData{
			HttpRequest: req,
		},
	})
	assert.Equal(structs.WorkflowExecutionStatus_Success, result.ExecutionStatus)
	jsonField := result.TriggerData[0]["json"].(map[string]interface{})
	assert.Equal("Ada", jsonField["Name"])
}

func TestExecute_MissingRequiredField(t *testing.T) {
	assert := require.New(t)
	req := newFormRequest(false, url.Values{
		"Name": []string{"Ada"},
	})
	defer fasthttp.ReleaseRequest(req)

	result := (&FormTrigger{}).Execute(context.Background(), &structs.NodeExecuteInput{
		Params: sampleFormTriggerNode(),
		AdditionalData: &structs.WorkflowExecuteAdditionalData{
			HttpRequest: req,
		},
	})
	assert.Equal(structs.WorkflowExecutionStatus_Failed, result.ExecutionStatus)
	assert.NotEmpty(result.Errors)
}

func TestExecute_NumberField(t *testing.T) {
	assert := require.New(t)
	node := sampleFormTriggerNode()
	node.Parameters["formFields"] = map[string]interface{}{
		"values": []interface{}{
			map[string]interface{}{
				"fieldLabel": "Age",
				"fieldType":  "number",
			},
		},
	}
	req := newFormRequest(false, url.Values{"Age": []string{"42"}})
	defer fasthttp.ReleaseRequest(req)

	result := (&FormTrigger{}).Execute(context.Background(), &structs.NodeExecuteInput{
		Params: node,
		AdditionalData: &structs.WorkflowExecuteAdditionalData{
			HttpRequest: req,
		},
	})
	assert.Equal(structs.WorkflowExecutionStatus_Success, result.ExecutionStatus)
	jsonField := result.TriggerData[0]["json"].(map[string]interface{})
	assert.Equal(float64(42), jsonField["Age"])
}
