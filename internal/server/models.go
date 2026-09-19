package server

import (
	"encoding/json"
	"net/http"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/registry"
)

// ModelsHandler implements the OpenAI-compatible GET /v1/models endpoint so
// standard OpenAI SDKs and tooling can discover the routable models. It lists
// the special "auto" routing sentinel plus every enabled registry model (which
// clients may pin via the request's `model` field).
func ModelsHandler(eng *engine.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := openai.ModelList{
			Object: "list",
			Data: []openai.ModelObject{
				{ID: "auto", Object: "model", OwnedBy: "sluss"},
			},
		}
		if eng != nil && eng.Registry != nil {
			if snap, err := eng.Registry.Active(); err == nil {
				created := snap.CreatedAt().Unix()
				for _, m := range snap.EnabledModelsWithCapabilities(registry.Capabilities{}) {
					list.Data = append(list.Data, openai.ModelObject{
						ID:      m.ID,
						Object:  "model",
						Created: created,
						OwnedBy: m.ProviderID,
					})
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	}
}
