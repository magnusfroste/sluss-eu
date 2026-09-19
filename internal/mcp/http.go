package mcp

import (
	"io"
	"net/http"
)

// HTTPHandler serves the server over MCP Streamable HTTP: a client POSTs a
// single JSON-RPC message and receives the JSON-RPC response as the body. All of
// tokenizer's tools are request/response (no server-initiated streaming), so a
// plain JSON body is a valid Streamable-HTTP response and needs no SSE.
//
// A GET returns 405 (this endpoint has no server-to-client stream to open). The
// caller is expected to wrap this handler with authentication.
func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 8*1024*1024))
		if err != nil {
			http.Error(w, "could not read request body", http.StatusBadRequest)
			return
		}
		resp, ok := s.HandleMessage(r.Context(), body)
		if !ok {
			// Notification: acknowledge with 202 and no body.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})
}
