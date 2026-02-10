// Package keepalived provides keepalived configuration rendering and management.
package keepalived

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/platform/exec"
)

// TemplateData holds data for keepalived config template.
type TemplateData struct {
	Role            string
	Interface       string
	VirtualRouterID int
	Priority        int
	AdvertInt       int
	VIP             string
	PeerIP          string
	SelfIP          string
	Preempt         bool
	PreemptDelay    int
	HealthMode      string
	CheckInterval   int
	CheckScript     string
	TrackWeight     int
	AgentBinary     string
}

const keepalivedTemplate = `# Gateway Agent Keepalived Configuration
# Role: {{ .Role }}
# Generated at: {{ now }}

global_defs {
    script_user root
    enable_script_security
}

vrrp_script chk_gateway {
    script "{{ .AgentBinary }} check --mode={{ .HealthMode }}"
    interval {{ .CheckInterval }}
    weight {{ .TrackWeight }}
    fall 3
    rise 2
    user root
    init_fail
}

vrrp_instance GATEWAY {
    state BACKUP
    interface {{ .Interface }}
    virtual_router_id {{ .VirtualRouterID }}
    priority {{ .Priority }}
    advert_int {{ .AdvertInt }}
    {{ if .Preempt }}preempt_delay {{ .PreemptDelay }}{{ else }}nopreempt{{ end }}

    authentication {
        auth_type PASS
        auth_pass gateway
    }

    {{ if .PeerIP }}
    unicast_src_ip {{ .SelfIP }}
    unicast_peer {
        {{ .PeerIP }}
    }
    {{ end }}

    virtual_ipaddress {
        {{ .VIP }}/32 dev {{ .Interface }}
    }

    track_script {
        chk_gateway
    }

    notify_master "{{ .AgentBinary }} notify MASTER"
    notify_backup "{{ .AgentBinary }} notify BACKUP"
    notify_fault  "{{ .AgentBinary }} notify FAULT"
}
`

// Renderer handles keepalived configuration rendering.
type Renderer struct {
	cfg *config.Config
}

// NewRenderer creates a new keepalived config renderer.
func NewRenderer(cfg *config.Config) *Renderer {
	return &Renderer{cfg: cfg}
}

// Render generates the keepalived configuration.
func (r *Renderer) Render() (string, error) {
	data := r.buildTemplateData()

	funcMap := template.FuncMap{
		"now": func() string {
			return time.Now().Format(time.RFC3339)
		},
	}

	tmpl, err := template.New("keepalived").Funcs(funcMap).Parse(keepalivedTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

func (r *Renderer) buildTemplateData() *TemplateData {
	// Determine track weight based on role
	// Secondary uses negative weight to drop priority when unhealthy
	trackWeight := 0
	if r.cfg.Role == config.RoleSecondary {
		trackWeight = -200 // Enough to drop below primary's priority
	}

	// Find agent binary path
	agentBinary := FindAgentBinary()
	// Force absolute path for reliability in generated config
	if !strings.HasPrefix(agentBinary, "/") {
		if abs, err := filepath.Abs(agentBinary); err == nil {
			agentBinary = abs
		} else {
			// Fallback to standard path if we can't determine absolute path
			agentBinary = "/gateway-agent/gateway-agent"
		}
	}

	return &TemplateData{
		Role:            string(r.cfg.Role),
		Interface:       r.cfg.LAN.Iface,
		VirtualRouterID: r.cfg.Keepalived.VRID,
		Priority:        r.cfg.GetPriority(),
		AdvertInt:       r.cfg.Keepalived.AdvertInt,
		VIP:             r.cfg.LAN.VIP,
		PeerIP:          r.cfg.Routers.PeerIP,
		SelfIP:          r.cfg.Routers.SelfIP,
		Preempt:         r.cfg.Failover.Preempt,
		PreemptDelay:    r.cfg.Failover.PreemptDelaySec,
		HealthMode:      string(r.cfg.Health.Mode),
		CheckInterval:   r.cfg.Health.IntervalSec,
		CheckScript:     fmt.Sprintf("%s check --mode=%s", agentBinary, r.cfg.Health.Mode),
		TrackWeight:     trackWeight,
		AgentBinary:     agentBinary,
	}
}

func FindAgentBinary() string {
	// Standard installation paths for gateway-agent
	paths := []string{
		"/gateway-agent/gateway-agent",
		"/etc/gateway-agent/gateway-agent",
		"/usr/bin/gateway-agent",
		"/usr/local/bin/gateway-agent",
	}
	// Try to get executable path of current running process if we are the agent
	if exe, err := os.Executable(); err == nil {
		if strings.Contains(exe, "gateway-agent") {
			return exe
		}
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Check in PATH as fallback
	if path, err := exec.Which("gateway-agent"); err == nil {
		return path
	}
	// Default to standard location
	return "/gateway-agent/gateway-agent"
}
