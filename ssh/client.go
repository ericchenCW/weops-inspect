package sshclient

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client wraps SSH connection parameters.
type Client struct {
	User           string
	ConnectTimeout time.Duration
	ExecTimeout    time.Duration
	config         *ssh.ClientConfig
}

// New creates a new SSH client with the given user and timeouts.
func New(user string, connectTimeout, execTimeout time.Duration) (*Client, error) {
	authMethods, err := buildAuthMethods()
	if err != nil {
		return nil, fmt.Errorf("failed to configure SSH auth: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         connectTimeout,
	}

	return &Client{
		User:           user,
		ConnectTimeout: connectTimeout,
		ExecTimeout:    execTimeout,
		config:         config,
	}, nil
}

// Run connects to the given host and executes the command, returning stdout.
func (c *Client) Run(host, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.ExecTimeout)
	defer cancel()
	return c.RunContext(ctx, host, command)
}

// RunContext connects and executes with a context for cancellation/timeout.
func (c *Client) RunContext(ctx context.Context, host, command string) (string, error) {
	addr := host
	if !strings.Contains(addr, ":") {
		addr = addr + ":22"
	}

	conn, err := ssh.Dial("tcp", addr, c.config)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s: %w", host, err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session %s: %w", host, err)
	}
	defer session.Close()

	type result struct {
		output []byte
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := session.CombinedOutput(command)
		ch <- result{out, err}
	}()

	select {
	case <-ctx.Done():
		session.Close()
		return "", fmt.Errorf("ssh exec timeout on %s: %w", host, ctx.Err())
	case r := <-ch:
		if r.err != nil {
			// Return output even on error (exit code != 0 is common for status checks)
			return string(r.output), r.err
		}
		return string(r.output), nil
	}
}

// buildAuthMethods tries SSH agent first, then default key files.
func buildAuthMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	// Try default key files
	home, err := os.UserHomeDir()
	if err == nil {
		keyFiles := []string{"id_rsa", "id_ecdsa", "id_ed25519"}
		for _, name := range keyFiles {
			keyPath := filepath.Join(home, ".ssh", name)
			key, err := os.ReadFile(keyPath)
			if err != nil {
				continue
			}
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				continue
			}
			methods = append(methods, ssh.PublicKeys(signer))
		}
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH auth methods available: no agent or key files found")
	}
	return methods, nil
}
