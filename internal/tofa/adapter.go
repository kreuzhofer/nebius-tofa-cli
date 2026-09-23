package tofa

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"sync"
	"time"
)

const maxAdapterBody = 16 << 20

type requestAdapter struct {
	endpoint string
	token    string
	server   *http.Server
	cancel   context.CancelFunc
	context  context.Context
	done     chan error
}

func (a *App) startAdapter(ctx context.Context, project, key, selectedModel string) (*requestAdapter, error) {
	endpoint := a.Endpoint
	if endpoint == "" {
		endpoint = Endpoint
	}
	upstream, err := url.Parse(endpoint + "/responses")
	if err != nil || upstream.Host == "" || upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" || (upstream.Scheme != "http" && upstream.Scheme != "https") {
		return nil, errors.New("invalid adapter upstream endpoint")
	}
	query := url.Values{"ai_project_id": {project}}
	upstream.RawQuery = query.Encode()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, errors.New("could not generate adapter credential")
	}
	listen := a.Listen
	if listen == nil {
		listen = net.Listen
	}
	listener, err := listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("could not start loopback request adapter")
	}
	ctx, cancel := context.WithCancel(ctx)
	adapter := &requestAdapter{endpoint: "http://" + listener.Addr().String(), token: hex.EncodeToString(secret), cancel: cancel, context: ctx, done: make(chan error, 1)}
	var noticeMu sync.Mutex
	notice := func(message string) {
		if selectedModel != "" {
			noticeMu.Lock()
			defer noticeMu.Unlock()
			fmt.Fprintln(a.Out, "Desktop request failed:", message)
		}
	}
	transport := http.DefaultTransport
	if a.HTTP != nil && a.HTTP.Transport != nil {
		transport = a.HTTP.Transport
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			target := *upstream
			request.Out.URL = &target
			request.Out.Host = upstream.Host
			request.Out.Header = make(http.Header)
			request.Out.Header.Set("Authorization", "Bearer "+key)
			request.Out.Header.Set("Content-Type", "application/json")
			request.Out.Header.Set("Accept", "text/event-stream, application/json")
			request.Out.GetBody = nil
			request.Out.Trailer = nil
			request.Out.TransferEncoding = nil
		},
		Transport:     transport,
		FlushInterval: -1,
		ErrorLog:      log.New(io.Discard, "", 0),
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			notice("upstream connection failed; request was not retried")
			http.Error(writer, "request adapter: upstream connection failed; request was not retried", http.StatusBadGateway)
		},
		ModifyResponse: func(response *http.Response) error {
			if response.StatusCode >= 400 {
				notice(fmt.Sprintf("upstream HTTP %d", response.StatusCode))
			}
			if response.StatusCode >= 300 && response.StatusCode < 400 {
				return errors.New("upstream redirect rejected")
			}
			return nil
		},
	}
	var approvalNotice sync.Once
	var titleNotice sync.Once
	adapter.server = &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    32 << 10,
		ErrorLog:          log.New(io.Discard, "", 0),
		BaseContext:       func(net.Listener) context.Context { return ctx },
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if subtle.ConstantTimeCompare([]byte(request.Header.Get("Authorization")), []byte("Bearer "+adapter.token)) != 1 {
				http.Error(writer, "request adapter: unauthorized", http.StatusUnauthorized)
				return
			}
			if request.URL.Path != "/responses" || request.URL.RawPath != "" || request.URL.RawQuery != "" {
				notice("unsupported route (including auxiliary/compaction endpoints)")
				http.Error(writer, "request adapter: unsupported route", http.StatusNotFound)
				return
			}
			if request.Method != http.MethodPost {
				writer.Header().Set("Allow", http.MethodPost)
				http.Error(writer, "request adapter: POST required", http.StatusMethodNotAllowed)
				return
			}
			if request.Header.Get("Content-Encoding") != "" && request.Header.Get("Content-Encoding") != "identity" {
				http.Error(writer, "request adapter: encoded requests are unsupported", http.StatusUnsupportedMediaType)
				return
			}
			body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxAdapterBody))
			if err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					http.Error(writer, "request adapter: request exceeds 16 MiB", http.StatusRequestEntityTooLarge)
				} else {
					http.Error(writer, "request adapter: could not read request", http.StatusBadRequest)
				}
				return
			}
			body, err = normalizeHistory(body)
			if err != nil {
				http.Error(writer, "request adapter: expected a JSON object", http.StatusBadRequest)
				return
			}
			if selectedModel != "" {
				var payload struct {
					Model string `json:"model"`
				}
				if json.Unmarshal(body, &payload) != nil || payload.Model != selectedModel {
					notice("unsupported model; only the explicitly selected model is routed")
					http.Error(writer, "request adapter: unsupported model; request was not sent upstream", http.StatusBadRequest)
					return
				}
			}
			body, titleAdapted, err := adaptDesktopTitle(body)
			if err != nil {
				notice(err.Error())
				http.Error(writer, "request adapter: "+err.Error(), http.StatusBadRequest)
				return
			}
			if titleAdapted {
				titleNotice.Do(func() {
					noticeMu.Lock()
					defer noticeMu.Unlock()
					fmt.Fprintln(a.Out, "Request adapter: Kimi-K3 desktop title schema moved to final-answer instructions; tools retained, desktop still validates the title.")
				})
			}
			body, adapted, err := adaptApprovalReview(body)
			if err != nil {
				notice(err.Error())
				http.Error(writer, "request adapter: "+err.Error(), http.StatusBadRequest)
				return
			}
			if adapted {
				approvalNotice.Do(func() {
					noticeMu.Lock()
					defer noticeMu.Unlock()
					fmt.Fprintln(a.Out, "Request adapter: Kimi-K3 approval-review schema moved to final-answer instructions; Codex still validates the decision.")
				})
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			request.ContentLength = int64(len(body))
			proxy.ServeHTTP(writer, request)
		}),
	}
	fmt.Fprintln(a.Out, "Route: per-launch Responses request adapter (assistant-history repair).")
	go func() {
		err := adapter.server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		if err != nil {
			cancel()
		}
		adapter.done <- err
	}()
	return adapter, nil
}

func (adapter *requestAdapter) close() error {
	adapter.cancel()
	if err := adapter.server.Close(); err != nil {
		return errors.New("request adapter could not close its connections")
	}
	return nil
}

func normalizeHistory(body []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil || payload == nil {
		return nil, errors.New("invalid JSON object")
	}
	var items []json.RawMessage
	if json.Unmarshal(payload["input"], &items) != nil {
		return body, nil
	}
	changed := false
	for index, raw := range items {
		var item map[string]json.RawMessage
		if json.Unmarshal(raw, &item) != nil || string(item["type"]) != `"message"` || string(item["role"]) != `"assistant"` {
			continue
		}
		var content []json.RawMessage
		if json.Unmarshal(item["content"], &content) != nil || content == nil {
			continue
		}
		itemChanged := false
		if _, exists := item["status"]; !exists {
			item["status"] = json.RawMessage(`"completed"`)
			itemChanged = true
		}
		for partIndex, partRaw := range content {
			var part map[string]json.RawMessage
			if json.Unmarshal(partRaw, &part) != nil || string(part["type"]) != `"output_text"` {
				continue
			}
			if _, exists := part["annotations"]; !exists {
				part["annotations"] = json.RawMessage(`[]`)
				content[partIndex], _ = json.Marshal(part)
				itemChanged = true
			}
		}
		if itemChanged {
			item["content"], _ = json.Marshal(content)
			items[index], _ = json.Marshal(item)
			changed = true
		}
	}
	if !changed {
		return body, nil
	}
	payload["input"], _ = json.Marshal(items)
	return json.Marshal(payload)
}

// Codex 0.155.1 guardian-reviewer/src/assessment.rs. This non-strict schema
// guides generation; Codex independently parses the decision and gates execution.
const guardianOutputSchema = `{"type":"object","additionalProperties":false,"properties":{"risk_level":{"type":"string","enum":["low","medium","high","critical"]},"user_authorization":{"type":"string","enum":["unknown","low","medium","high"]},"outcome":{"type":"string","enum":["allow","deny"]},"rationale":{"type":"string"}},"required":["outcome"]}`

func adaptApprovalReview(body []byte) ([]byte, bool, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, err
	}
	var model string
	json.Unmarshal(payload["model"], &model)
	if model != "moonshotai/Kimi-K3" {
		return body, false, nil
	}
	var textOptions, format map[string]json.RawMessage
	if json.Unmarshal(payload["text"], &textOptions) != nil || json.Unmarshal(textOptions["format"], &format) != nil {
		return body, false, nil
	}
	var formatType string
	json.Unmarshal(format["type"], &formatType)
	if formatType != "json_schema" {
		return body, false, nil
	}
	var choice string
	json.Unmarshal(payload["tool_choice"], &choice)
	if choice == "none" {
		return body, false, nil
	}
	var toolDefinitions []map[string]json.RawMessage
	var tools []json.RawMessage
	if len(payload["tools"]) == 0 || string(payload["tools"]) == "null" || (json.Unmarshal(payload["tools"], &tools) == nil && len(tools) == 0) {
		return body, false, nil
	}

	unsupported := errors.New("Kimi-K3 tools with json_schema are unsupported except the recognized non-strict Codex approval review; request was not sent upstream")
	if json.Unmarshal(payload["tools"], &toolDefinitions) != nil {
		return nil, false, unsupported
	}
	for field := range format {
		switch field {
		case "type", "schema", "strict":
		case "name":
			var name *string
			if json.Unmarshal(format[field], &name) != nil || name == nil {
				return nil, false, unsupported
			}
		default:
			return nil, false, unsupported
		}
	}
	var strict *bool
	if json.Unmarshal(format["strict"], &strict) != nil || strict == nil || *strict {
		return nil, false, unsupported
	}
	var schema, expected any
	if json.Unmarshal(format["schema"], &schema) != nil {
		return nil, false, unsupported
	}
	json.Unmarshal([]byte(guardianOutputSchema), &expected)
	if !reflect.DeepEqual(schema, expected) {
		return nil, false, unsupported
	}
	if choice != "auto" {
		return nil, false, unsupported
	}
	allowed := map[string]bool{"exec_command": true, "write_stdin": true, "view_image": true}
	if len(toolDefinitions) != len(allowed) {
		return nil, false, unsupported
	}
	for _, tool := range toolDefinitions {
		var name, kind string
		json.Unmarshal(tool["name"], &name)
		json.Unmarshal(tool["type"], &kind)
		if kind != "function" || !allowed[name] {
			return nil, false, unsupported
		}
		delete(allowed, name)
	}
	var instructions *string
	if json.Unmarshal(payload["instructions"], &instructions) != nil || instructions == nil {
		return nil, false, unsupported
	}
	*instructions += "\n\nWhen you are ready to give your final answer, return JSON matching this schema:\n" + string(format["schema"])
	payload["instructions"], _ = json.Marshal(instructions)
	delete(textOptions, "format")
	payload["text"], _ = json.Marshal(textOptions)
	result, err := json.Marshal(payload)
	return result, true, err
}
