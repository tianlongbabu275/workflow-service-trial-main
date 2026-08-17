package form_trigger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/sugerio/workflow-service-trial/shared/structs"
)

func sampleFormTriggerNode() *structs.WorkflowNode {
	return &structs.WorkflowNode{
		ID:          "3c19b4a9-acdd-488b-90e3-ea8b4e1534b3",
		Name:        "n8n Form Trigger",
		Type:        Name,
		TypeVersion: 2.1,
		WebhookId:   "e5427436-0460-4fb7-baa3-f8ee2a2b7d92",
		Parameters: map[string]interface{}{
			"path":            "e5427436-0460-4fb7-baa3-f8ee2a2b7d92",
			"formTitle":       "Contact Us Now Customize",
			"formDescription": "We'll get back to you soon",
			"formFields": map[string]interface{}{
				"values": []interface{}{
					map[string]interface{}{
						"fieldLabel":    "Name",
						"requiredField": true,
					},
					map[string]interface{}{
						"fieldLabel":    "Email",
						"fieldType":     "email",
						"requiredField": true,
					},
					map[string]interface{}{
						"fieldLabel":    "Address",
						"requiredField": true,
					},
					map[string]interface{}{
						"fieldLabel":    "Hobby",
						"fieldType":     "dropdown",
						"requiredField": true,
						"fieldOptions": map[string]interface{}{
							"values": []interface{}{
								map[string]interface{}{"option": "basketball"},
								map[string]interface{}{"option": "soccer"},
							},
						},
					},
				},
			},
			"options": map[string]interface{}{},
		},
	}
}

func TestRenderHTML_SampleContactForm(t *testing.T) {
	assert := require.New(t)
	action := "/workflow/public/webhook/workflow/wf-1/node/node-1?webhookId=e5427436-0460-4fb7-baa3-f8ee2a2b7d92&isTest=false"

	html, err := RenderHTML(sampleFormTriggerNode(), action)
	assert.NoError(err)
	assert.Contains(html, "<!DOCTYPE html>")
	assert.Contains(html, "Contact Us Now Customize")
	assert.Contains(html, "get back to you soon")
	assert.Contains(html, `method="post"`)
	assert.Contains(html, "/workflow/public/webhook/workflow/wf-1/node/node-1")
	assert.Contains(html, "webhookId=e5427436-0460-4fb7-baa3-f8ee2a2b7d92")
	assert.Contains(html, "isTest=false")
	assert.Contains(html, `name="Name"`)
	assert.Contains(html, `name="Email"`)
	assert.Contains(html, `type="email"`)
	assert.Contains(html, `name="Address"`)
	assert.Contains(html, `name="Hobby"`)
	assert.Contains(html, "basketball")
	assert.Contains(html, "soccer")
	assert.Contains(html, "required")
	assert.Contains(html, "Submit")
}

func TestRenderHTML_EscapesUserContent(t *testing.T) {
	assert := require.New(t)
	node := sampleFormTriggerNode()
	node.Parameters["formTitle"] = `<script>alert("xss")</script>`

	html, err := RenderHTML(node, "/form")
	assert.NoError(err)
	assert.NotContains(html, `<script>alert("xss")</script>`)
	assert.Contains(html, "&lt;script&gt;")
}

func TestRenderHTML_WritesPreviewFile(t *testing.T) {
	assert := require.New(t)
	html, err := RenderHTML(sampleFormTriggerNode(), "/demo-form")
	assert.NoError(err)

	previewDir := filepath.Join("testdata")
	assert.NoError(os.MkdirAll(previewDir, 0o755))
	previewPath := filepath.Join(previewDir, "contact-us.preview.html")
	assert.NoError(os.WriteFile(previewPath, []byte(html), 0o644))

	written, err := os.ReadFile(previewPath)
	assert.NoError(err)
	assert.True(strings.Contains(string(written), "Contact Us Now Customize"))
	t.Logf("Open this file in a browser to preview the form UI: %s", previewPath)
}
