package proxy

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
)

// WrapConfig configures a stdio wrap session around a child MCP server.
type WrapConfig struct {
	Command     string
	Args        []string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	Logger      *log.Logger
	Interceptor Interceptor
}

// RunWrap launches command and proxies MCP stdio traffic through the interceptor.
func RunWrap(ctx context.Context, cfg WrapConfig) error {
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(cfg.Stderr, "mcp-breaker: ", log.LstdFlags)
	}
	if cfg.Interceptor == nil {
		cfg.Interceptor = PassThroughInterceptor{}
	}

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Stderr = cfg.Stderr

	childIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	childOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	session := &Session{
		ClientIn:      cfg.Stdin,
		ClientOut:     cfg.Stdout,
		ServerIn:      childIn,
		ServerOut:     childOut,
		Logger:        cfg.Logger,
		Intercept:     cfg.Interceptor,
		CloseServerIn: childIn.Close,
	}

	runErr := session.Run(ctx)
	waitErr := cmd.Wait()
	if runErr != nil && runErr != io.EOF && runErr != context.Canceled {
		return runErr
	}
	if waitErr != nil && ctx.Err() == nil {
		return waitErr
	}
	return nil
}
