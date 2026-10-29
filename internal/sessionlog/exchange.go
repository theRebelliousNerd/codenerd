package sessionlog

import "time"

// ExchangeSchema names the exchange file format.
const ExchangeSchema = "codenerd.exchange/1"

// TextEncoding is written into every file so a reader needs no other document.
const TextEncoding = `Long text is stored as an array of its lines: join with "\n" to restore it exactly. ` +
	`In wire bodies a string that contained a newline is split the same way and its JSON pointer is listed in lines_paths; ` +
	`every other array is an array on the wire.`

// Outcomes of an exchange.
const (
	OutcomeOK      = "ok"
	OutcomeError   = "error"
	OutcomeRefused = "refused" // the broker refused the request; nothing reached the provider
)

// Exchange is one model API exchange: what went out, what came back, what it
// cost, and the decisions that shaped the request.
type Exchange struct {
	Schema       string      `json:"schema"`
	TextEncoding string      `json:"text_encoding"`
	Number       int         `json:"number"`
	Purpose      string      `json:"purpose"`
	Correlation  Correlation `json:"correlation"`

	Provider  string    `json:"provider,omitempty"`
	Model     string    `json:"model,omitempty"`
	Method    string    `json:"method,omitempty"`
	Started   time.Time `json:"started"`
	LatencyMS int64     `json:"latency_ms"`
	Outcome   string    `json:"outcome"`
	Error     string    `json:"error,omitempty"`

	// Wire holds every provider round trip of the call in order: a retry is
	// another entry. Empty when the client sent nothing through a captured
	// transport, in which case LogicalRequest carries what the broker was
	// asked to send.
	Wire           []Wire          `json:"wire,omitempty"`
	LogicalRequest *LogicalRequest `json:"logical_request,omitempty"`

	Response  *Response    `json:"response,omitempty"`
	Usage     *Usage       `json:"usage,omitempty"`
	Admission any          `json:"admission,omitempty"`
	Decisions *DecisionSet `json:"decisions,omitempty"`
}

// Correlation ties an exchange to the work that made it.
type Correlation struct {
	Session    string `json:"session"`
	Turn       int    `json:"turn,omitempty"`
	Task       string `json:"task,omitempty"`
	Agent      string `json:"agent"`
	Role       string `json:"role,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	Phase      string `json:"phase,omitempty"`
	Credential string `json:"credential_alias,omitempty"`
}

// Wire is one provider round trip.
type Wire struct {
	Attempt    int           `json:"attempt"`
	Transport  string        `json:"transport"` // "http" or "exec"
	Started    time.Time     `json:"started"`
	DurationMS int64         `json:"duration_ms"`
	Request    WireRequest   `json:"request"`
	Response   *WireResponse `json:"response,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// WireRequest is the request as sent. For an exec transport Method is "exec",
// URL is the command, and Body is {"args": [...], "stdin": ...}.
type WireRequest struct {
	Method     string              `json:"method"`
	URL        string              `json:"url"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       any                 `json:"body,omitempty"`
	LinesPaths []string            `json:"lines_paths,omitempty"`
	BodyNote   string              `json:"body_note,omitempty"`
	BodyBytes  int                 `json:"body_bytes,omitempty"`
}

// WireResponse is the response as received.
type WireResponse struct {
	Status     int                 `json:"status"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       any                 `json:"body,omitempty"`
	LinesPaths []string            `json:"lines_paths,omitempty"`
	// BodyNote says why Body is empty when it is: a streamed body is not kept
	// as events; the assembled result is Exchange.Response.
	BodyNote  string `json:"body_note,omitempty"`
	BodyBytes int    `json:"body_bytes,omitempty"`
}

// LogicalRequest is the request in the broker's own terms, recorded only when
// no wire capture exists for the call.
type LogicalRequest struct {
	System     []string         `json:"system,omitempty"`
	User       []string         `json:"user,omitempty"`
	Messages   []LogicalMessage `json:"messages,omitempty"`
	Tools      any              `json:"tools,omitempty"`
	LinesPaths []string         `json:"lines_paths,omitempty"`
}

// LogicalMessage is one message of a logical request.
type LogicalMessage struct {
	Role        string   `json:"role"`
	Text        []string `json:"text,omitempty"`
	ToolCalls   any      `json:"tool_calls,omitempty"`
	ToolResults any      `json:"tool_results,omitempty"`
	LinesPaths  []string `json:"lines_paths,omitempty"`
}

// Response is the final form of the answer: a stream assembled, with its stop
// reason.
type Response struct {
	Text       []string   `json:"text,omitempty"`
	Thoughts   []string   `json:"thoughts,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason,omitempty"`
	Streamed   bool       `json:"streamed,omitempty"`
	// Extra carries anything else the method returned (a structured result).
	Extra any `json:"extra,omitempty"`
}

// ToolCall is a tool invocation the model requested.
type ToolCall struct {
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name"`
	Input      any      `json:"input,omitempty"`
	LinesPaths []string `json:"lines_paths,omitempty"`
	ArgsError  string   `json:"args_error,omitempty"`
}

// Usage is what the provider reported, and what the usage price table says it
// cost.
type Usage struct {
	InputTokens      int    `json:"input_tokens"`
	CacheReadTokens  int    `json:"cache_read_tokens"`
	CacheWriteTokens int    `json:"cache_write_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	ThinkingTokens   int    `json:"thinking_tokens,omitempty"`
	Status           string `json:"status,omitempty"` // reported | partial | unreported
	Attempts         int    `json:"attempts,omitempty"`
	Price            *Price `json:"price,omitempty"`
}

// Price is the cost of the usage at the usage price table's rates.
type Price struct {
	USD     float64 `json:"usd"`
	Version string  `json:"table_version,omitempty"`
	// Priced is false when the table has no entry for the model; USD is then 0
	// and means "unknown", not "free".
	Priced bool `json:"priced"`
}
