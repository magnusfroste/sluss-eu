package mcp

import (
	"bufio"
	"context"
	"io"
	"sync"
)

// ServeStdio runs the server over a newline-delimited JSON-RPC stream, the
// transport local dev agents (e.g. Claude Code) use when they launch the binary
// and talk over stdin/stdout. Each inbound line is one JSON-RPC message; each
// response is written as one line. It returns when the reader reaches EOF or the
// context is cancelled.
func ServeStdio(ctx context.Context, s *Server, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Allow large messages (default 64K token is too small for some payloads).
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var writeMu sync.Mutex
	writeLine := func(b []byte) error {
		if b == nil {
			return nil
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		if _, err := out.Write(b); err != nil {
			return err
		}
		_, err := out.Write([]byte("\n"))
		return err
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// Copy: scanner reuses its buffer, and the handler may outlive this pass.
		msg := make([]byte, len(line))
		copy(msg, line)
		resp, ok := s.HandleMessage(ctx, msg)
		if ok {
			if err := writeLine(resp); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
