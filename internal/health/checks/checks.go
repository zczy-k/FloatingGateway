// Package checks provides health check implementations.
package checks

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/platform/exec"
)

// Result represents the result of a health check.
type Result struct {
	Type      string        `json:"type"`
	Target    string        `json:"target"`
	OK        bool          `json:"ok"`
	LatencyMs int64         `json:"latency_ms"`
	ErrorCode string        `json:"error_code,omitempty"`
	Message   string        `json:"message,omitempty"`
	Duration  time.Duration `json:"-"`
}

// Checker is the interface for health checks.
type Checker interface {
	Check(ctx context.Context) *Result
	Type() string
	Target() string
}

// NewChecker creates a checker from config.
func NewChecker(cfg config.CheckConfig) (Checker, error) {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	switch cfg.Type {
	case "ping":
		if cfg.Target == "" {
			return nil, fmt.Errorf("ping check requires target")
		}
		return &PingChecker{target: cfg.Target, timeout: timeout}, nil
	
	case "dns":
		if cfg.Resolver == "" || cfg.Domain == "" {
			return nil, fmt.Errorf("dns check requires resolver and domain")
		}
		return &DNSChecker{resolver: cfg.Resolver, domain: cfg.Domain, timeout: timeout}, nil
	
	case "tcp":
		if cfg.Target == "" || cfg.Port == 0 {
			return nil, fmt.Errorf("tcp check requires target and port")
		}
		return &TCPChecker{target: cfg.Target, port: cfg.Port, timeout: timeout}, nil
	
	case "http":
		url := cfg.URL
		if url == "" && cfg.Target != "" {
			url = cfg.Target
		}
		if url == "" {
			return nil, fmt.Errorf("http check requires url or target")
		}
		return &HTTPChecker{url: url, timeout: timeout}, nil
	
	default:
		return nil, fmt.Errorf("unknown check type: %s", cfg.Type)
	}
}

// PingChecker performs ICMP ping checks.
type PingChecker struct {
	target  string
	timeout time.Duration
}

func (c *PingChecker) Type() string   { return "ping" }
func (c *PingChecker) Target() string { return c.target }

func (c *PingChecker) Check(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{
		Type:   c.Type(),
		Target: c.target,
	}

	// Use system ping command
	var args []string
	// Detect ping version (Linux vs BSD/macOS)
	if exec.CommandExists("ping") {
		// Try Linux style first (-c count, -W timeout in seconds)
		// We also add -n to avoid DNS resolution during ping
		args = []string{"-c", "1", "-W", fmt.Sprintf("%d", int(c.timeout.Seconds())), "-n", c.target}
	} else {
		result.OK = false
		result.ErrorCode = "PING_CMD_NOT_FOUND"
		result.Message = "system ping command not found"
		return result
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout+1*time.Second) // Give it a bit more time than the -W timeout
	defer cancel()

	cmdResult := exec.Run(timeoutCtx, "ping", args...)
	result.Duration = time.Since(start)
	result.LatencyMs = result.Duration.Milliseconds()

	if cmdResult.Success() {
		result.OK = true
		result.Message = "ping successful"
		// Try to extract RTT from output
		// We use a more robust regex-like parsing for different ping formats
		output := cmdResult.Stdout
		if strings.Contains(output, "time=") {
			// Extract value between "time=" and " ms"
			idx := strings.Index(output, "time=")
			if idx != -1 {
				sub := output[idx+5:]
				endIdx := strings.Index(sub, " ")
				if endIdx != -1 {
					var rtt float64
					fmt.Sscanf(sub[:endIdx], "%f", &rtt)
					if rtt > 0 {
						result.LatencyMs = int64(rtt)
					}
				}
			}
		}
	} else {
		// Fallback for some busybox versions where -W might not be supported or behaves differently
		if cmdResult.ExitCode != 0 && strings.Contains(cmdResult.Stderr, "invalid option") {
			// Retry with simpler args
			args = []string{"-c", "1", c.target}
			cmdResult = exec.Run(timeoutCtx, "ping", args...)
			if cmdResult.Success() {
				result.OK = true
				return result
			}
		}

		result.OK = false
		result.ErrorCode = "PING_FAILED"
		result.Message = fmt.Sprintf("ping failed: %s", cmdResult.Combined())
	}

	return result
}

// DNSChecker performs DNS resolution checks.
type DNSChecker struct {
	resolver string
	domain   string
	timeout  time.Duration
}

func (c *DNSChecker) Type() string   { return "dns" }
func (c *DNSChecker) Target() string { return fmt.Sprintf("%s@%s", c.domain, c.resolver) }

func (c *DNSChecker) Check(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{
		Type:   c.Type(),
		Target: c.Target(),
	}

	// Use custom resolver
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: c.timeout}
			return d.DialContext(ctx, "udp", net.JoinHostPort(c.resolver, "53"))
		},
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	addrs, err := resolver.LookupHost(timeoutCtx, c.domain)
	result.Duration = time.Since(start)
	result.LatencyMs = result.Duration.Milliseconds()

	if err != nil {
		result.OK = false
		result.ErrorCode = "DNS_FAILED"
		result.Message = fmt.Sprintf("DNS lookup failed: %v", err)
	} else if len(addrs) == 0 {
		result.OK = false
		result.ErrorCode = "DNS_NO_RESULT"
		result.Message = "DNS lookup returned no addresses"
	} else {
		result.OK = true
		result.Message = fmt.Sprintf("resolved to %s", strings.Join(addrs, ", "))
	}

	return result
}

// TCPChecker performs TCP connection checks.
type TCPChecker struct {
	target  string
	port    int
	timeout time.Duration
}

func (c *TCPChecker) Type() string   { return "tcp" }
func (c *TCPChecker) Target() string { return fmt.Sprintf("%s:%d", c.target, c.port) }

func (c *TCPChecker) Check(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{
		Type:   c.Type(),
		Target: c.Target(),
	}

	dialer := net.Dialer{Timeout: c.timeout}
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := dialer.DialContext(timeoutCtx, "tcp", c.Target())
	result.Duration = time.Since(start)
	result.LatencyMs = result.Duration.Milliseconds()

	if err != nil {
		result.OK = false
		result.ErrorCode = "TCP_FAILED"
		result.Message = fmt.Sprintf("TCP connect failed: %v", err)
	} else {
		conn.Close()
		result.OK = true
		result.Message = "TCP connect successful"
	}

	return result
}

// HTTPChecker performs HTTP request checks.
type HTTPChecker struct {
	url     string
	timeout time.Duration
}

func (c *HTTPChecker) Type() string   { return "http" }
func (c *HTTPChecker) Target() string { return c.url }

func (c *HTTPChecker) Check(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{
		Type:   c.Type(),
		Target: c.url,
	}

	client := &http.Client{
		Timeout: c.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, "GET", c.url, nil)
	if err != nil {
		result.OK = false
		result.ErrorCode = "HTTP_INVALID_REQUEST"
		result.Message = fmt.Sprintf("invalid request: %v", err)
		return result
	}

	req.Header.Set("User-Agent", "gateway-agent/1.0")

	resp, err := client.Do(req)
	result.Duration = time.Since(start)
	result.LatencyMs = result.Duration.Milliseconds()

	if err != nil {
		result.OK = false
		result.ErrorCode = "HTTP_FAILED"
		result.Message = fmt.Sprintf("HTTP request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	// Consider 2xx and 3xx as success
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		result.OK = true
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		result.OK = false
		result.ErrorCode = fmt.Sprintf("HTTP_%d", resp.StatusCode)
		result.Message = fmt.Sprintf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	return result
}

// RunAll runs all checkers and returns results.
func RunAll(ctx context.Context, checkers []Checker) []*Result {
	results := make([]*Result, len(checkers))
	for i, checker := range checkers {
		results[i] = checker.Check(ctx)
	}
	return results
}

// CreateCheckers creates checkers from config.
func CreateCheckers(configs []config.CheckConfig) ([]Checker, error) {
	checkers := make([]Checker, 0, len(configs))
	for i, cfg := range configs {
		checker, err := NewChecker(cfg)
		if err != nil {
			return nil, fmt.Errorf("check[%d]: %w", i, err)
		}
		checkers = append(checkers, checker)
	}
	return checkers, nil
}
