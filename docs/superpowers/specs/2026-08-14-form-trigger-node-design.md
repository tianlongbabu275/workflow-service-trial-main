# Form Trigger Node 详细设计

日期：2026-08-14  
项目：`workflow-service-trial`  
需求文档：`Suger Backend Project 1_ Form Trigger Node.docx`

本文是实现规格。对外讲解稿见 [`docs/Form-Trigger-解题思路.md`](../../Form-Trigger-解题思路.md)（第 1 节是仓库模块梳理）。两份文档都保留：规格用于对照实现，讲解稿用于答辩。

## 1. 背景与目标

在现有 `workflow-service` 上实现 n8n Form Trigger 节点。用户打开表单 URL 时看到 HTML 表单；提交后启动工作流，并把字段作为第一个节点的输出传给下游（示例中是 Code 节点）。

**成功标准**

1. `GET` 同一 Webhook URL 返回根据节点配置渲染的 HTML 表单，**不**启动 workflow。
2. `POST` 同一 URL 解析表单字段，启动 workflow，下游能读到字段值。
3. 复用已有 Webhook API：`HandleWebhook`，不新建公开路由。
4. 单节点测试 + 工作流端到端测试都覆盖 GET 渲染和 POST 提交。

## 2. 需求拆解

Form Trigger 有两个互斥语义，共用一个 URL：

```
BaseURL/workflow/public/webhook/workflow/:workflowId/node/:nodeId?webhookId=:webhookId
```

可选 query：`isTest=true|false`（与现有 webhook 一致）。

### 2.1 GET：渲染表单

根据节点 `parameters` 生成 HTML：

| 配置 | 示例 | HTML 表现 |
| --- | --- | --- |
| `formTitle` | Contact Us Now Customize | `<h1>` |
| `formDescription` | We'll get back to you soon | 描述段落 |
| `formFields.values[]` | Name / Email / Address / Hobby | 对应 input / select |
| `fieldLabel` | Name | label + input name |
| `fieldType` | 缺省 text；email；dropdown | input type 或 `<select>` |
| `requiredField` | true | `required` |
| `fieldOptions.values[].option` | basketball / soccer | `<option>` |

### 2.2 POST：提交并触发执行

解析 body，构造 trigger 输出 json，调用现有 `core.RunWorkflow`。responseMode 走现有 webhook 逻辑。

示例工作流节点：

1. `n8n-nodes-base.formTrigger`（webhookId = path = `e5427436-...`）
2. `n8n-nodes-base.code`（给每条 json 加 `myNewField = 1`）

### 2.3 明确不做（本期）

n8n 后续版本有、但本 trial 配置 / `node.json` 未要求的能力：文件上传、自定义 CSS、Basic Auth / n8n User Auth、Bot 忽略、IP 白名单、多页 Form Ending、n8n 前端 SPA。`responseNode` / `lastNode` 复用现有 webhook 即可，不单独实现表单成功页。

## 3. 现状与缺口

已有：

- `nodes/form_trigger/node.json`、`form.svg`（无 `node.go`，未注册）
- Webhook 节点 + `HandleWebhook` + `webhook_entity` 注册/注销
- Code 节点可直接用于示例工作流

关键缺口：

1. `nodes.go` 没有 import `form_trigger`，节点对执行器不可见。
2. `getWebhookMethod()` 读 `parameters.httpMethod`，缺省 **GET**。Form Trigger 没有该字段，会把实体注册成 GET。结果：GET 会错误地 `RunWorkflow`；POST 会被 method 校验拒绝。
3. `HandleWebhook` 对任何方法都执行 workflow，没有「只渲染 HTML」分支。
4. `GetWebhookEntity` 只按 `webhookPath` 取第一条，不按 HTTP method 过滤。若注册 GET+POST 两行会命中错误 method。
5. `GetWebhookIdAndNodeIdInWorkflow` 只认 `n8n-nodes-base.webhook`。

`webhook_entity` 主键是 `(webhookPath, method)`，技术上可以存两行；本设计**不走双实体**，避免改查找语义。

## 4. 方案对比

### 方案 A（推荐）：POST 实体 + GET 短路

- `getWebhookMethod`：`n8n-nodes-base.formTrigger` 固定返回 `POST`。
- `HandleWebhook`：加载 node 后，若 `type == formTrigger && method == GET`，渲染 HTML 并返回，**不**做 method 严格匹配、**不** `RunWorkflow`。
- GET 能找到实体，是因为同一 `webhookId` 的 POST 行已存在（激活或 test webhook 注册之后）。
- POST 走现有 `onReceived` / `lastNode` / `responseNode`。

优点：改动面小，完全满足「复用 webhook API」。  
缺点：GET 不对应独立 webhook 行，与 n8n 的 setup/default 双 webhook 不完全同构。对本 trial 足够。

### 方案 B：双实体 GET + POST

按 n8n `webhooks[]` 写两行。必须改 `GetWebhookEntity` 按 method 过滤，并改 `GetWorkflowWebhooks` 按 spec 展开多 method。更完整，core 回归风险更大。本期不采用。

### 方案 C：独立 Form 路由

与需求「URL 与 Webhook API 相同」冲突。不采用。

## 5. 架构

```
Browser/Test
    │  GET / POST  同一路径
    ▼
HandleWebhook
    │  校验 webhookId，加载 webhook_entity / workflow / node
    ├─ GET  + formTrigger ──► RenderFormHTML ──► text/html
    └─ POST + formTrigger ──► 现有 responseMode 分支
                                  │
                                  ▼
                             RunWorkflow
                                  │
                                  ▼
                        FormTrigger.Execute
                          解析表单 → json
                                  │
                                  ▼
                             Code / 其他节点
```

公开路由不变：

```go
fiberApp.All("/workflow/public/webhook/workflow/:workflowId/node/:nodeId", service.HandleWebhook)
```

## 6. 组件设计

### 6.1 节点包 `service/workflow_service/nodes/form_trigger/`

| 文件 | 职责 |
| --- | --- |
| `node.go` | 实现 `core.NodeObject`；`Execute` 只处理 POST 解析 |
| `render.go` | 从 `WorkflowNode.Parameters` 生成 HTML |
| `form.html` | `html/template`，自动转义 |
| `node.json` | 已有，不改结构 |
| `form.svg` | 已有，embed 注册 |

常量：

```go
const (
    Category = structs.CategoryTrigger
    Name     = "n8n-nodes-base.formTrigger"
)
```

`init()`：`GenerateSpec` + `core.Register` + `core.RegisterEmbedIcons`。

`nodes.go` 增加：

```go
_ "github.com/sugerio/workflow-service-trial/service/workflow_service/nodes/form_trigger"
```

### 6.2 参数结构

```go
type FormFieldOption struct {
    Option string `json:"option"`
}

type FormFieldOptions struct {
    Values []FormFieldOption `json:"values"`
}

type FormField struct {
    FieldLabel    string            `json:"fieldLabel"`
    FieldType     string            `json:"fieldType"` // 空则 text
    RequiredField bool              `json:"requiredField"`
    Multiselect   bool              `json:"multiselect"`
    FieldOptions  *FormFieldOptions `json:"fieldOptions,omitempty"`
}

type FormFields struct {
    Values []FormField `json:"values"`
}

type FormTriggerParameters struct {
    Path            string     `json:"path"`
    FormTitle       string     `json:"formTitle"`
    FormDescription string     `json:"formDescription"`
    FormFields      FormFields `json:"formFields"`
    ResponseMode    string     `json:"responseMode"`
    Options         map[string]interface{} `json:"options"`
}
```

支持的 `fieldType`：`text`（默认）、`email`、`number`、`date`、`password`、`textarea`、`dropdown`。

说明：仓库里的 `node.json` options 没有 `email`，但需求示例 JSON 使用了 `"fieldType": "email"`，必须支持。

### 6.3 HTML 渲染

模板要点：

- `Content-Type: text/html; charset=utf-8`
- `<form method="POST" action="当前 URL（含 webhookId、isTest）">`
- 每个字段 `name` 使用 `fieldLabel`（typeVersion 2.1，n8n 用 label 作 json key）
- `dropdown` → `<select>`；`multiselect=true` 时 `multiple`
- `requiredField` → `required`
- Submit 按钮文案默认 `Submit`
- 所有配置文本走 `html/template` 转义，防止 XSS

GET **不**执行 workflow，即使节点配置了 `responseMode=lastNode`。

### 6.4 Execute 输出

与 n8n `prepareFormReturnItem` 对齐（精简版）：

```json
{
  "json": {
    "Name": "Ada",
    "Email": "ada@example.com",
    "Address": "1 Market St",
    "Hobby": "basketball",
    "submittedAt": "2026-08-14T14:00:00.000Z",
    "formMode": "production"
  }
}
```

规则：

- json key = `fieldLabel`
- `text` / `email`：trim
- `number`：转成 JSON number
- 缺省或空非必填：`null`
- `formMode`：`isTest=true` → `test`，否则 `production`
- `submittedAt`：UTC ISO-8601
- Trigger 节点通过 `GenerateSuccessResponse(triggerData, nil)` 返回，`TriggerData` 形状与 webhook 节点一致：`[]NodeSingleData{{ "json": ... }}`

Body 解析顺序：

1. `multipart/form-data`
2. `application/x-www-form-urlencoded`
3. `application/json` 对象（方便单节点单测；e2e 用 form POST）

必填字段缺失 → `GenerateFailedResponse`。

### 6.5 HandleWebhook 改动

在「找到 webhookNode、校验 webhookId 属于该 node」之后、method 校验之前插入：

```go
if webhookNode.Type == formtrigger.Name && method == http.MethodGet {
    html, err := formtrigger.RenderHTML(webhookNode, ctx.OriginalURL())
    if err != nil {
        return HandleInternalServerErrorWithTrace(ctx, err)
    }
    ctx.Type("html", "utf-8")
    return ctx.Status(fiber.StatusOK).SendString(html)
}
```

然后保持 `webhookEntity.Method != method` 校验。Form Trigger 实体 method=POST，GET 已在上面返回；POST 通过校验进入 `RunWorkflow`。

SNS subscription 分支保持不动（Form 不会走到）。

### 6.6 getWebhookMethod

```go
if node.Type == "n8n-nodes-base.formTrigger" {
    return http.MethodPost
}
```

放在读 `httpMethod` 参数之前。现有 webhook / sugerNotificationEventTrigger 行为不变。

### 6.7 测试辅助

扩展 `api/test_util.go`：

- `GetWebhookIdAndNodeIdInWorkflow` 同时识别 `n8n-nodes-base.formTrigger`，或新增 `GetFormTriggerIdAndNodeIdInWorkflow`（推荐后者，避免改现有 webhook 测试语义）。
- `CallWebhookHTML_Testing(...)`：GET，断言 200，返回 `response.Body` 字符串。
- `CallWebhookForm_Testing(...)`：POST `application/x-www-form-urlencoded`，返回完整 `APIGatewayProxyResponse`（onReceived 时 body 是 JSON，不是 HTML）。

## 7. 数据流（示例工作流）

1. `CreateWorkflow` 写入示例 JSON（node id / webhookId 使用需求附件）。
2. `ActivateWorkflow` → `RegisterWebhook` 写入 `webhook_entity(path=webhookId, method=POST)`。
3. GET → HTML，包含 title、四个字段、Hobby 两个 option。
4. POST `Name=Ada&Email=ada@example.com&Address=1+Market+St&Hobby=basketball`
5. `FormTrigger.Execute` 产出 json。
6. Code 节点增加 `myNewField: 1`。
7. 默认 `responseMode=onReceived`：HTTP 立即返回 `{"executionId","message":"Workflow was started"}`。测试再用 GetExecution 断言 Code 输出。

Test webhook：`ManualRun` → `RegisterTestWebhooksIfAny` → `isTest=true`，路径为 `webhookId/test`，与现有 webhook 相同。

## 8. 错误处理

| 场景 | 行为 |
| --- | --- |
| 缺 webhookId | 现有 400 |
| webhook 未注册 | 现有 404 |
| GET 但 workflow 未激活且无 test webhook | 404（与 webhook 一致） |
| POST method 与实体不符（非 form GET 短路） | 现有 400 |
| POST 缺必填字段 | 节点失败；onReceived 仍可能先返回 started，lastNode 返回 workflow error。单节点测试直接断言 Failed。工作流测试对必填校验以单节点为主。 |
| 未知 fieldType | 按 `text` 渲染 / 按字符串解析 |
| HTML 渲染失败 | 500 |

## 9. 测试计划

文件：`service/workflow_service/nodes_test/form_trigger_test.go`  
夹具：`nodes_test/test_files/form-trigger-e2e.json`（需求附件 JSON，空 id 按现有 CreateWorkflow 测试习惯补齐 org 字段）。

运行：

```bash
go test -v service/workflow_service/nodes_test/init_test.go service/workflow_service/nodes_test/form_trigger_test.go
```

### 单节点

1. Spec：`DisplayName == "n8n Form Trigger"`，`Name == n8n-nodes-base.formTrigger`。
2. Generate：executor 能取出 spec。
3. RenderHTML：给定示例 parameters，HTML 含 title、description、`name="Name"`、`type="email"`、`basketball`/`soccer`、`required`、`method="post"`。
4. Execute：构造 fasthttp POST form body，断言 json 字段、`formMode`、`submittedAt` 非空。
5. Execute 缺必填：Failed。

### 工作流

1. 创建 + 激活 → `webhook_entity` 1 条，method=POST。
2. GET production URL → 200 HTML，含表单字段。
3. POST form → 200，body 含 `executionId`；用 `WaitWorkflowExecution_Testing` 轮询到终态后，Code 结果含表单字段且 `myNewField=1`。
4. 未激活时 GET/POST → 非 200。
5. 激活后再 deactivate → 实体删除，GET/POST 失败。
6. ManualRun + `isTest=true`：GET 出表单，POST 能跑通；DeleteTestWebhook 后失败。

不改现有 `webhook_test.go` 期望。

## 10. 实现顺序

1. `form_trigger` 包：参数解析、RenderHTML、Execute + 单节点测试。
2. `nodes.go` 注册。
3. `getWebhookMethod` 特例。
4. `HandleWebhook` GET 短路。
5. test_util 辅助函数。
6. 工作流 e2e。

## 11. 风险与决策记录

1. **GET 不注册独立实体**：依赖 POST 实体存在。与「表单只在 workflow 激活或 test listen 时可用」一致。
2. **POST 默认 JSON 响应，不是 n8n 成功页**：需求要求复用 webhook API；`onReceived` 返回 `Workflow was started`。若产品要「Your response has been recorded」HTML，可作为后续，用 `options.respondWithOptions`。
3. **字段 name 用 fieldLabel 而非 `field-0`**：匹配 typeVersion 2.1 输出 key，测试更直观。
4. **email 类型**：按需求示例支持，不改 `node.json` 以保持与 n8n 描述文件同步。
