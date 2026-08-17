package form_trigger

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sugerio/workflow-service-trial/service/workflow_service/core"
	"github.com/sugerio/workflow-service-trial/shared/structs"
	"github.com/valyala/fasthttp"
)

const (
	Category = structs.CategoryTrigger
	Name     = "n8n-nodes-base.formTrigger"
)

var (
	//go:embed node.json
	rawJson []byte

	//go:embed form.svg
	icon []byte
)

type FormTrigger struct {
	spec *structs.WorkflowNodeSpec
}

func init() {
	node := &FormTrigger{
		spec: &structs.WorkflowNodeSpec{},
	}
	node.spec.JsonConfig = rawJson
	node.spec.GenerateSpec()
	core.Register(node)
	core.RegisterEmbedIcons(Name, icon)
}

func (n *FormTrigger) Category() structs.NodeObjectCategory {
	return Category
}

func (n *FormTrigger) Name() string {
	return Name
}

func (n *FormTrigger) DefaultSpec() interface{} {
	return n.spec
}

func (n *FormTrigger) Execute(ctx context.Context, input *structs.NodeExecuteInput) *structs.NodeExecutionResult {
	nodeName := Name
	if input != nil && input.Params != nil && input.Params.Name != "" {
		nodeName = input.Params.Name
	}
	if input == nil || input.Params == nil {
		return core.GenerateFailedResponse(nodeName, fmt.Errorf("missing node execute input"))
	}
	if input.AdditionalData == nil || input.AdditionalData.HttpRequest == nil {
		return core.GenerateFailedResponse(nodeName, fmt.Errorf("missing http request"))
	}

	params, err := ParseParameters(input.Params)
	if err != nil {
		return core.GenerateFailedResponse(nodeName, err)
	}

	submitted, err := parseSubmittedValues(input.AdditionalData.HttpRequest)
	if err != nil {
		return core.GenerateFailedResponse(nodeName, err)
	}

	output := make(map[string]interface{}, len(params.FormFields.Values)+2)
	for _, field := range params.FormFields.Values {
		raw, exists := submitted[field.FieldLabel]
		if field.RequiredField && (!exists || strings.TrimSpace(fmt.Sprint(raw)) == "") {
			return core.GenerateFailedResponse(nodeName, fmt.Errorf("missing required field: %s", field.FieldLabel))
		}
		output[field.FieldLabel] = normalizeFieldValue(field, raw, exists)
	}

	output["submittedAt"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	output["formMode"] = formModeFromRequest(input.AdditionalData.HttpRequest)

	return core.GenerateSuccessResponse(structs.NodeData{
		{
			"json": output,
		},
	}, []structs.NodeData{})
}

func formModeFromRequest(request *fasthttp.Request) string {
	if string(request.URI().QueryArgs().Peek("isTest")) == "true" {
		return "test"
	}
	return "production"
}

func parseSubmittedValues(request *fasthttp.Request) (map[string]interface{}, error) {
	contentType := string(request.Header.ContentType())
	body := request.Body()
	values := map[string]interface{}{}

	switch {
	case strings.HasPrefix(contentType, "application/json"):
		if len(body) == 0 {
			return values, nil
		}
		if err := json.Unmarshal(body, &values); err != nil {
			return nil, fmt.Errorf("invalid json form body: %w", err)
		}
		return values, nil
	case strings.HasPrefix(contentType, "multipart/form-data"):
		form, err := request.MultipartForm()
		if err != nil {
			return nil, err
		}
		for key, items := range form.Value {
			if len(items) == 1 {
				values[key] = items[0]
			} else {
				values[key] = items
			}
		}
		return values, nil
	default:
		formValues, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		// PostArgs is populated for urlencoded requests copied from Fiber.
		request.PostArgs().VisitAll(func(key, value []byte) {
			formValues.Set(string(key), string(value))
		})
		for key, items := range formValues {
			if len(items) == 1 {
				values[key] = items[0]
			} else if len(items) > 1 {
				values[key] = items
			}
		}
		return values, nil
	}
}

func normalizeFieldValue(field FormField, raw interface{}, exists bool) interface{} {
	if !exists || raw == nil {
		return nil
	}
	str := strings.TrimSpace(fmt.Sprint(raw))
	if str == "" || str == "<nil>" {
		return nil
	}
	switch field.FieldType {
	case "email", "text", "textarea", "password", "date":
		return str
	case "number":
		number, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return str
		}
		return number
	default:
		return str
	}
}
