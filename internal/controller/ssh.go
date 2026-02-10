// Package controller provides the central management functionality for the floating gateway.
package controller

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConfig holds SSH connection parameters.
type SSHConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	User       string `yaml:"user"`
	Password   string `yaml:"password"`
	KeyFile    string `yaml:"key_file"`
	KeyData    []byte `yaml:"-"`
	Passphrase string `yaml:"passphrase"`
	Timeout    int    `yaml:"timeout"`
}

// SSHClient wraps an SSH connection for remote operations.
type SSHClient struct {
	config *SSHConfig
	client *ssh.Client
}

// NewSSHClient creates a new SSH client.
func NewSSHClient(cfg *SSHConfig) *SSHClient {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}
	return &SSHClient{config: cfg}
}

// Connect establishes the SSH connection.
func (c *SSHClient) Connect() error {
	authMethods := []ssh.AuthMethod{}

	// Try key-based auth first
	if c.config.KeyFile != "" || len(c.config.KeyData) > 0 {
		signer, err := c.getKeySigner()
		if err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	// Fallback to password auth
	if c.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(c.config.Password))
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("no authentication method available")
	}

	sshConfig := &ssh.ClientConfig{
		User:            c.config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: proper host key verification
		Timeout:         time.Duration(c.config.Timeout) * time.Second,
	}

	addr := net.JoinHostPort(c.config.Host, fmt.Sprintf("%d", c.config.Port))
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("ssh dial: %w", err)
	}

	c.client = client
	return nil
}

// getKeySigner returns an SSH signer from key file or key data.
func (c *SSHClient) getKeySigner() (ssh.Signer, error) {
	var keyData []byte
	var err error

	if len(c.config.KeyData) > 0 {
		keyData = c.config.KeyData
	} else if c.config.KeyFile != "" {
		// Expand ~ to home directory
		keyPath := c.config.KeyFile
		if strings.HasPrefix(keyPath, "~") {
			home, _ := os.UserHomeDir()
			keyPath = filepath.Join(home, keyPath[1:])
		}
		keyData, err = os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no key data or key file provided")
	}

	var signer ssh.Signer
	if c.config.Passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(c.config.Passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return signer, nil
}

// Close closes the SSH connection.
func (c *SSHClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Run executes a command and returns stdout, stderr, and error.
func (c *SSHClient) Run(cmd string) (stdout, stderr string, err error) {
	if c.client == nil {
		return "", "", fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(cmd)
	return stdoutBuf.String(), stderrBuf.String(), err
}

// RunCombined executes a command and returns combined output.
func (c *SSHClient) RunCombined(cmd string) (string, error) {
	stdout, stderr, err := c.Run(cmd)
	output := stdout + stderr
	return strings.TrimSpace(output), err
}

// RunStdout executes a command and returns only stdout.
func (c *SSHClient) RunStdout(cmd string) (string, error) {
	stdout, _, err := c.Run(cmd)
	return strings.TrimSpace(stdout), err
}

// RunScript executes multiple commands as a script.
func (c *SSHClient) RunScript(script string) (string, error) {
	return c.RunCombined(script)
}

// Upload copies data to a remote file.
func (c *SSHClient) Upload(data []byte, remotePath string, mode os.FileMode) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	// Use scp protocol
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		filename := filepath.Base(remotePath)
		fmt.Fprintf(w, "C%04o %d %s\n", mode, len(data), filename)
		w.Write(data)
		fmt.Fprint(w, "\x00")
	}()

	dir := filepath.Dir(remotePath)
	return session.Run(fmt.Sprintf("scp -t %s", dir))
}

// UploadFile copies a local file to remote path.
func (c *SSHClient) UploadFile(localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read local file: %w", err)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}

	return c.Upload(data, remotePath, info.Mode())
}

// Download reads a remote file.
func (c *SSHClient) Download(remotePath string) ([]byte, error) {
	stdout, _, err := c.Run(fmt.Sprintf("cat '%s'", remotePath))
	if err != nil {
		return nil, err
	}
	return []byte(stdout), nil
}

// Exists checks if a remote file exists.
func (c *SSHClient) Exists(remotePath string) bool {
	_, _, err := c.Run(fmt.Sprintf("test -e '%s'", remotePath))
	return err == nil
}

// MkdirAll creates a directory tree on remote.
func (c *SSHClient) MkdirAll(dir string) error {
	_, err := c.RunCombined(fmt.Sprintf("mkdir -p '%s'", dir))
	return err
}

// WriteFile writes data to a remote file using cat for binary safety.
// This method is compatible with OpenWrt/busybox which may not have base64.
func (c *SSHClient) WriteFile(remotePath string, data []byte, mode os.FileMode) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("get stdin pipe: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		defer stdin.Close()
		_, err := stdin.Write(data)
		errCh <- err
	}()

	// Use cat to write binary data directly - works on all systems including busybox
	cmd := fmt.Sprintf("cat > '%s' && chmod %04o '%s'", remotePath, mode, remotePath)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("run command: %w", err)
	}

	if err := <-errCh; err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

// RemoveFile removes a remote file.
func (c *SSHClient) RemoveFile(remotePath string) error {
	_, err := c.RunCombined(fmt.Sprintf("rm -f '%s'", remotePath))
	return err
}

// IsConnected returns true if connected.
func (c *SSHClient) IsConnected() bool {
	return c.client != nil
}

// Host returns the connection host.
func (c *SSHClient) Host() string {
	return c.config.Host
}
