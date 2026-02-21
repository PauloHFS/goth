package llm

import (
	"encoding/json"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type CompletionRequest struct {
	Model             string          `json:"model"`
	Messages          []Message       `json:"messages"`
	Temperature       float64         `json:"temperature,omitempty"`
	TopP              float64         `json:"top_p,omitempty"`
	MaxTokens         int             `json:"max_tokens,omitempty"`
	Stream            bool            `json:"stream,omitempty"`
	Stop              []string        `json:"stop,omitempty"`
	PresencePenalty   float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty  float64         `json:"frequency_penalty,omitempty"`
	Tools             []Tool          `json:"tools,omitempty"`
	ToolChoice        *ToolChoice     `json:"tool_choice,omitempty"`
	ResponseFormat    *ResponseFormat `json:"response_format,omitempty"`
	User              string          `json:"user,omitempty"`
	N                 int             `json:"n,omitempty"`
	Seed              *int            `json:"seed,omitempty"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls,omitempty"`
	LogitBias         map[string]int  `json:"logit_bias,omitempty"`
}

type CompletionResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
}

type Choice struct {
	Index        int       `json:"index"`
	Message      *Message  `json:"message,omitempty"`
	Delta        *Message  `json:"delta,omitempty"`
	FinishReason string    `json:"finish_reason,omitempty"`
	LogProbs     *LogProbs `json:"logprobs,omitempty"`
}

type LogProbs struct {
	Content []LogProbToken `json:"content,omitempty"`
}

type LogProbToken struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []StreamChoice `json:"choices"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
	Usage             *Usage         `json:"usage,omitempty"`
}

type StreamChoice struct {
	Index        int       `json:"index"`
	Delta        Message   `json:"delta"`
	FinishReason string    `json:"finish_reason,omitempty"`
	LogProbs     *LogProbs `json:"logprobs,omitempty"`
}

type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      bool                   `json:"strict,omitempty"`
}

type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolChoice struct {
	Type     string              `json:"type"`
	Function *ToolChoiceFunction `json:"function,omitempty"`
}

type ToolChoiceFunction struct {
	Name string `json:"name"`
}

type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

type JSONSchema struct {
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
}

type EmbeddingRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	Dimensions     *int        `json:"dimensions,omitempty"`
}

type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type OllamaStreamChunk struct {
	Model           string    `json:"model"`
	CreatedAt       time.Time `json:"created_at"`
	Message         Message   `json:"message"`
	Done            bool      `json:"done"`
	DoneReason      string    `json:"done_reason,omitempty"`
	TotalDuration   int64     `json:"total_duration,omitempty"`
	LoadDuration    int64     `json:"load_duration,omitempty"`
	PromptEvalCount int       `json:"prompt_eval_count,omitempty"`
	EvalCount       int       `json:"eval_count,omitempty"`
	EvalDuration    int64     `json:"eval_duration,omitempty"`
}

func (o *OllamaStreamChunk) ToStreamChunk() StreamChunk {
	choice := StreamChoice{
		Delta: o.Message,
	}
	if o.Done {
		choice.FinishReason = o.DoneReason
		if o.DoneReason == "" {
			choice.FinishReason = "stop"
		}
	}
	return StreamChunk{
		Model:   o.Model,
		Created: o.CreatedAt.Unix(),
		Choices: []StreamChoice{choice},
	}
}

type JSONRichMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Name    string          `json:"name,omitempty"`
}

func (m *Message) UnmarshalJSON(data []byte) error {
	var rich JSONRichMessage
	if err := json.Unmarshal(data, &rich); err != nil {
		return err
	}

	m.Role = Role(rich.Role)
	m.Name = rich.Name

	if string(rich.Content) == "null" {
		m.Content = ""
	} else {
		m.Content = string(rich.Content)
	}

	return nil
}
