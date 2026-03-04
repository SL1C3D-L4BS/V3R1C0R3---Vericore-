package main

import (
	"encoding/json"
	"net/http"

	"v3r1c0r3.local/mcp-proxy"
)

// mcpContextRequest is the body for POST /api/v1/mcp/context (simulated MCP resource fetch).
type mcpContextRequest struct {
	ResourceURI      string `json:"resource_uri"`
	SimulatedPayload string `json:"simulated_payload"`
}

// mcpContextResponse is the response for POST /api/v1/mcp/context.
type mcpContextResponse struct {
	ContextHash string `json:"context_hash"`
	ResourceURI string `json:"resource_uri"`
}

// mcpContextHandler handles POST /api/v1/mcp/context: registers the simulated payload in the MCP proxy cache and returns the context hash.
func mcpContextHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req mcpContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	contextHash := mcpproxy.RegisterContext([]byte(req.SimulatedPayload))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mcpContextResponse{
		ContextHash: contextHash,
		ResourceURI: req.ResourceURI,
	})
}
