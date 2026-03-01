package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pepebot-space/pepebot/pkg/logger"
)

type RemoteTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

type Client interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]RemoteTool, error)
	CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error)
	Close() error
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type stdioClient struct {
	command     string
	args        []string
	env         map[string]string
	protocol    string // auto | header | line
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	reader      *bufio.Reader
	mu          sync.Mutex
	requestID   int64
	initialized bool
}

func NewStdioClient(command string, args []string, env map[string]string) Client {
	return &stdioClient{command: command, args: args, env: env, protocol: "auto"}
}

func (c *stdioClient) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	if err := c.startProcess(ctx); err != nil {
		return err
	}

	if _, err := c.requestWithProtocolFallbackLocked("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "pepebot",
			"version": "0.5.5",
		},
	}); err != nil {
		return err
	}

	if err := c.notifyLocked("notifications/initialized", map[string]interface{}{}); err != nil {
		logger.DebugCF("mcp", "Initialized notification failed (continuing)", map[string]interface{}{
			"command": c.command,
			"error":   err.Error(),
		})
	}

	c.initialized = true
	return nil
}

func (c *stdioClient) ListTools(ctx context.Context) ([]RemoteTool, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := c.requestLocked("tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []RemoteTool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/list response: %w", err)
	}

	return result.Tools, nil
}

func (c *stdioClient) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if err := c.Initialize(ctx); err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := c.requestLocked("tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	return parseToolCallResult(raw), nil
}

func (c *stdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	c.initialized = false
	return nil
}

func (c *stdioClient) startProcess(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.command, c.args...)
	cmd.Stderr = os.Stderr

	mergedEnv := os.Environ()
	for k, v := range c.env {
		mergedEnv = append(mergedEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = mergedEnv

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP stdio server: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.reader = bufio.NewReader(stdout)

	logger.DebugCF("mcp", "Started MCP stdio server", map[string]interface{}{
		"command": c.command,
		"args":    c.args,
	})

	return nil
}

func (c *stdioClient) requestLocked(method string, params interface{}) (json.RawMessage, error) {
	return c.requestWithProtocolFallbackLocked(method, params)
}

func (c *stdioClient) requestWithProtocolFallbackLocked(method string, params interface{}) (json.RawMessage, error) {
	if c.protocol == "header" || c.protocol == "line" {
		return c.requestLockedWithMode(method, params, c.protocol, 30*time.Second)
	}

	if raw, err := c.requestLockedWithMode(method, params, "header", 8*time.Second); err == nil {
		c.protocol = "header"
		return raw, nil
	}

	logger.DebugCF("mcp", "Retrying MCP request with line protocol", map[string]interface{}{
		"command": c.command,
		"method":  method,
	})

	c.killProcessLocked()
	if err := c.startProcess(context.Background()); err != nil {
		return nil, err
	}

	raw, err := c.requestLockedWithMode(method, params, "line", 20*time.Second)
	if err != nil {
		return nil, err
	}
	c.protocol = "line"
	return raw, nil
}

func (c *stdioClient) requestLockedWithMode(method string, params interface{}, mode string, timeout time.Duration) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.requestID, 1)
	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if err := c.writeMessage(req, mode); err != nil {
		return nil, err
	}

	type rpcResult struct {
		raw json.RawMessage
		err error
	}
	resultCh := make(chan rpcResult, 1)

	go func() {
		for {
			payload, err := c.readMessage()
			if err != nil {
				resultCh <- rpcResult{err: err}
				return
			}

			var resp rpcResponse
			if err := json.Unmarshal(payload, &resp); err != nil {
				continue
			}

			if len(resp.ID) == 0 {
				continue
			}

			var gotID int64
			if err := json.Unmarshal(resp.ID, &gotID); err != nil {
				continue
			}
			if gotID != id {
				continue
			}

			if resp.Error != nil {
				resultCh <- rpcResult{err: fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)}
				return
			}

			resultCh <- rpcResult{raw: resp.Result}
			return
		}
	}()

	select {
	case res := <-resultCh:
		return res.raw, res.err
	case <-time.After(timeout):
		c.killProcessLocked()
		return nil, fmt.Errorf("mcp request timeout for method %s", method)
	}
}

func (c *stdioClient) notifyLocked(method string, params interface{}) error {
	req := rpcRequest{JSONRPC: "2.0", Method: method, Params: params}
	mode := c.protocol
	if mode == "auto" {
		mode = "header"
	}
	return c.writeMessage(req, mode)
}

func (c *stdioClient) writeMessage(v interface{}, mode string) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal mcp request: %w", err)
	}

	if mode == "line" {
		if _, err := c.stdin.Write(append(payload, '\n')); err != nil {
			return fmt.Errorf("failed to write mcp payload: %w", err)
		}
		return nil
	}

	headers := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	if _, err := c.stdin.Write([]byte(headers)); err != nil {
		return fmt.Errorf("failed to write mcp headers: %w", err)
	}
	if _, err := c.stdin.Write(payload); err != nil {
		return fmt.Errorf("failed to write mcp payload: %w", err)
	}

	return nil
}

func (c *stdioClient) readMessage() ([]byte, error) {
	firstLine, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read mcp response: %w", err)
	}
	trimmed := strings.TrimRight(firstLine, "\r\n")
	if strings.HasPrefix(trimmed, "{") {
		return []byte(trimmed), nil
	}

	contentLength := 0
	line := trimmed
	for {
		if line == "" {
			break
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "content-length:") {
			v := strings.TrimSpace(line[len("Content-Length:"):])
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid content-length: %w", err)
			}
			contentLength = n
		}

		nextLine, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read mcp header: %w", err)
		}
		line = strings.TrimRight(nextLine, "\r\n")
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("missing content-length in mcp response")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return nil, fmt.Errorf("failed to read mcp payload: %w", err)
	}
	return payload, nil
}

func (c *stdioClient) killProcessLocked() {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	c.cmd = nil
	c.stdin = nil
	c.stdout = nil
	c.reader = nil
	c.initialized = false
}

type httpClient struct {
	url         string
	headers     map[string]string
	httpClient  *http.Client
	requestID   int64
	initialized bool
	mu          sync.Mutex
}

func NewHTTPClient(url string, headers map[string]string) Client {
	return &httpClient{
		url:        url,
		headers:    headers,
		httpClient: &http.Client{},
	}
}

func (c *httpClient) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	_, err := c.requestLocked(ctx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "pepebot",
			"version": "0.5.5",
		},
	})
	if err != nil {
		return err
	}

	_, _ = c.requestLocked(ctx, "notifications/initialized", map[string]interface{}{})
	c.initialized = true
	return nil
}

func (c *httpClient) ListTools(ctx context.Context) ([]RemoteTool, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := c.requestLocked(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []RemoteTool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/list response: %w", err)
	}

	return result.Tools, nil
}

func (c *httpClient) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if err := c.Initialize(ctx); err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := c.requestLocked(ctx, "tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	return parseToolCallResult(raw), nil
}

func (c *httpClient) Close() error {
	return nil
}

func (c *httpClient) requestLocked(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.requestID, 1)
	reqBody := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp http transport returned %d: %s", resp.StatusCode, string(body))
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("invalid mcp http response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func parseToolCallResult(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent interface{} `json:"structuredContent"`
		IsError           bool        `json:"isError"`
	}

	if err := json.Unmarshal(raw, &parsed); err == nil {
		if len(parsed.Content) > 0 {
			parts := make([]string, 0, len(parsed.Content))
			for _, c := range parsed.Content {
				if strings.TrimSpace(c.Text) != "" {
					parts = append(parts, c.Text)
				}
			}
			if len(parts) > 0 {
				result := strings.Join(parts, "\n")
				if parsed.IsError {
					return "Error: " + result
				}
				return result
			}
		}

		if parsed.StructuredContent != nil {
			b, _ := json.Marshal(parsed.StructuredContent)
			if parsed.IsError {
				return "Error: " + string(b)
			}
			return string(b)
		}
	}

	return string(raw)
}
