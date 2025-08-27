package proc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Child struct {
	Cmd  *exec.Cmd
	Name string
	URL  string // e.g., public URL for cloudflared quick tunnel
}

type Supervisor struct {
	mu     sync.Mutex
	childs map[string]*Child
	log    func(format string, args ...any)
}

func NewSupervisor(logger func(string, ...any)) *Supervisor {
	return &Supervisor{childs: map[string]*Child{}, log: logger}
}

func (s *Supervisor) Start(name string, cmd *exec.Cmd) (*Child, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.childs[name]; ok {
		return nil, fmt.Errorf("%s already started", name)
	}
	stderr, _ := cmd.StderrPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	ch := &Child{Cmd: cmd, Name: name}
	s.childs[name] = ch
	// Log pipes
	go s.pipeLogs(name, stdout)
	go s.pipeLogs(name, stderr)
	return ch, nil
}

func (s *Supervisor) pipeLogs(name string, r io.ReadCloser) {
	defer r.Close()
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		s.log("[%s] %s", name, line)
	}
}

func (s *Supervisor) StopAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var first error
	for name, ch := range s.childs {
		if ch.Cmd.Process == nil {
			continue
		}
		s.log("stopping %s (pid=%d)", name, ch.Cmd.Process.Pid)
		if err := terminate(ch.Cmd); err != nil && first == nil {
			first = fmt.Errorf("%s: %w", name, err)
		}
	}
	// Wait briefly
	waitCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	for name, ch := range s.childs {
		done := make(chan error, 1)
		go func(c *exec.Cmd) { done <- c.Wait() }(ch.Cmd)
		select {
		case <-waitCtx.Done():
			_ = ch.Cmd.Process.Kill()
		case <-done:
		}
		delete(s.childs, name)
		s.log("stopped %s", name)
	}
	return first
}

func terminate(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return errors.New("no process")
	}
	// Try graceful interrupt, fallback to Kill
	if runtime.GOOS == "windows" {
		return cmd.Process.Kill()
	}
	// Send SIGTERM if possible
	return cmd.Process.Signal(os.Interrupt)
}
