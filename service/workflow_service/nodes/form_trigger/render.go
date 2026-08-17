package form_trigger

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/sugerio/workflow-service-trial/service/workflow_service/core"
	"github.com/sugerio/workflow-service-trial/shared/structs"
)

//go:embed form.html
var formTemplateRaw string

var formTemplate = template.Must(template.New("form").Parse(formTemplateRaw))

type (
	FormFieldOption struct {
		Option string `json:"option"`
	}

	FormFieldOptions struct {
		Values []FormFieldOption `json:"values"`
	}

	FormField struct {
		FieldLabel    string            `json:"fieldLabel"`
		FieldType     string            `json:"fieldType"`
		RequiredField bool              `json:"requiredField"`
		Multiselect   bool              `json:"multiselect"`
		FieldOptions  *FormFieldOptions `json:"fieldOptions,omitempty"`
	}

	FormFields struct {
		Values []FormField `json:"values"`
	}

	FormTriggerParameters struct {
		Path            string                 `json:"path"`
		FormTitle       string                 `json:"formTitle"`
		FormDescription string                 `json:"formDescription"`
		FormFields      FormFields             `json:"formFields"`
		ResponseMode    string                 `json:"responseMode"`
		Options         map[string]interface{} `json:"options"`
	}

	renderedField struct {
		ID          string
		FieldLabel  string
		FieldType   string
		InputType   string
		Required    bool
		Multiselect bool
		Options     []string
	}

	formView struct {
		FormTitle       string
		FormDescription string
		Action          string
		ButtonLabel     string
		Fields          []renderedField
	}
)

func ParseParameters(node *structs.WorkflowNode) (*FormTriggerParameters, error) {
	if node == nil {
		return nil, fmt.Errorf("form trigger node is nil")
	}
	params, err := core.ConvertInterfaceToType[FormTriggerParameters](node.Parameters)
	if err != nil {
		return nil, err
	}
	for i := range params.FormFields.Values {
		if strings.TrimSpace(params.FormFields.Values[i].FieldType) == "" {
			params.FormFields.Values[i].FieldType = "text"
		}
	}
	return params, nil
}

func inputTypeFor(fieldType string) string {
	switch fieldType {
	case "email", "number", "date", "password":
		return fieldType
	default:
		return "text"
	}
}

func RenderHTML(node *structs.WorkflowNode, action string) (string, error) {
	params, err := ParseParameters(node)
	if err != nil {
		return "", err
	}

	view := formView{
		FormTitle:       params.FormTitle,
		FormDescription: params.FormDescription,
		Action:          action,
		ButtonLabel:     "Submit",
		Fields:          make([]renderedField, 0, len(params.FormFields.Values)),
	}
	for i, field := range params.FormFields.Values {
		rendered := renderedField{
			ID:          fmt.Sprintf("field-%d", i),
			FieldLabel:  field.FieldLabel,
			FieldType:   field.FieldType,
			InputType:   inputTypeFor(field.FieldType),
			Required:    field.RequiredField,
			Multiselect: field.Multiselect,
		}
		if field.FieldOptions != nil {
			for _, option := range field.FieldOptions.Values {
				rendered.Options = append(rendered.Options, option.Option)
			}
		}
		view.Fields = append(view.Fields, rendered)
	}

	var buf bytes.Buffer
	if err := formTemplate.Execute(&buf, view); err != nil {
		return "", err
	}
	return buf.String(), nil
}
