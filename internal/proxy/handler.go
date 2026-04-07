package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/pdonorio/claude-context-proxy/internal/config"
)

// Upstream is the default Anthropic API base URL.
const Upstream = "https://api.anthropic.com"

// sseEventData holds the minimal JSON fields needed from SSE events.
type sseEventData struct {
	Type         string `json:"type"`
	ContentBlock struct {
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"content_block"`
}

// SSEInspector wraps an io.Reader and extracts tool names from SSE events inline.
// It is only instantiated when cfg.Inspect is true; zero overhead in the default path.
type SSEInspector struct {
	r     io.Reader
	buf   []byte
	Tools []string
}

// NewSSEInspector returns a new SSEInspector wrapping r.
func NewSSEInspector(r io.Reader) *SSEInspector { return &SSEInspector{r: r} }

func (s *SSEInspector) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if n > 0 {
		s.ingest(p[:n])
	}
	return n, err
}

func (s *SSEInspector) ingest(chunk []byte) {
	s.buf = append(s.buf, chunk...)
	for {
		idx := bytes.Index(s.buf, []byte("\n\n"))
		if idx < 0 {
			break
		}
		s.parseEvent(s.buf[:idx])
		s.buf = s.buf[idx+2:]
	}
}

func (s *SSEInspector) parseEvent(raw []byte) {
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		var ev sseEventData
		if json.Unmarshal(line[6:], &ev) != nil {
			continue
		}
		if ev.Type == "content_block_start" && ev.ContentBlock.Type == "tool_use" && ev.ContentBlock.Name != "" {
			s.Tools = append(s.Tools, ev.ContentBlock.Name)
		}
	}
}

// OnTokensFn is called after a response completes with extracted token counts.
type OnTokensFn func(input, output int64, path, model string, tools []string)

// Handler returns an http.HandlerFunc that reverse-proxies to targetURL.
// After each response, onTokens is called with the extracted token counts (if any).
func Handler(targetURL string, cfg *config.Config, onTokens OnTokensFn) http.HandlerFunc {
	client := &http.Client{Timeout: 0} // no timeout — streaming responses can be long
	return func(w http.ResponseWriter, r *http.Request) {
		target := targetURL + r.RequestURI

		// Buffer the request body to extract the model and then replay it.
		var bodyBuf []byte
		if r.Body != nil {
			var err error
			bodyBuf, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read request body: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		model := extractModel(cfg, bodyBuf)

		proxyReq, err := http.NewRequest(r.Method, target, io.NopCloser(bytes.NewReader(bodyBuf)))
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Copy all request headers verbatim.
		for k, vals := range r.Header {
			for _, v := range vals {
				proxyReq.Header.Add(k, v)
			}
		}

		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Extract token counts from response headers.
		inputTokens, _ := strconv.ParseInt(resp.Header.Get("X-Anthropic-Input-Tokens"), 10, 64)
		outputTokens, _ := strconv.ParseInt(resp.Header.Get("X-Anthropic-Output-Tokens"), 10, 64)

		// Copy response headers then status.
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)

		// Stream body; use SSEInspector when inspect mode is enabled.
		isSSE := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
		var inspector *SSEInspector
		if isSSE {
			bodyReader := io.Reader(resp.Body)
			if cfg.Inspect {
				inspector = NewSSEInspector(resp.Body)
				bodyReader = inspector
			}
			flusher, ok := w.(http.Flusher)
			buf := make([]byte, 4096)
			for {
				n, readErr := bodyReader.Read(buf)
				if n > 0 {
					if _, werr := w.Write(buf[:n]); werr != nil {
						// Client disconnected — stop streaming.
						break
					}
					if ok {
						flusher.Flush()
					}
				}
				if readErr == io.EOF || readErr != nil {
					break
				}
			}
		} else {
			// Non-streaming: copy body; ignore client-disconnect errors.
			_, _ = io.Copy(w, resp.Body)
		}

		if inputTokens > 0 || outputTokens > 0 {
			var tools []string
			if inspector != nil {
				tools = inspector.Tools
			}
			onTokens(inputTokens, outputTokens, r.URL.Path, model, tools)
		}
	}
}

// extractModel extracts the "model" field from a JSON request body.
func extractModel(cfg *config.Config, body []byte) string {
	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		return cfg.DefaultModel
	}
	if model, ok := data["model"].(string); ok && model != "" {
		return model
	}
	return cfg.DefaultModel
}
