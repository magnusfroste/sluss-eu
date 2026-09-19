package server

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/magnusfroste/sluss/internal/engine"
)

// chatModelOption is one entry in the chat page's model picker (ISSUE-098):
// enough for an admin to make an informed pin — tier and output price make the
// cost/CO₂ trade visible, egress shows whether a pin would leave the house.
type chatModelOption struct {
	ID            string  `json:"id"`
	Tier          string  `json:"tier"`
	Egress        string  `json:"egress"` // "local" | "cloud"
	OutUSDPerMTok float64 `json:"out_usd_per_mtok"`
}

// ChatModelsHandler lists the routable models for the chat model picker. The
// picker is an admin control-panel feature (verify a model end-to-end, demo the
// cost difference vs Auto); the endpoint is admin-gated and exposes only what
// the admin pages already show.
func ChatModelsHandler(eng *engine.Engine) http.HandlerFunc {
	tierOrder := map[string]int{"cheap": 0, "balanced": 1, "premium": 2}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var out []chatModelOption
		if eng != nil && eng.Registry != nil {
			if snap, err := eng.Registry.Active(); err == nil {
				for _, p := range snap.Providers() {
					for _, m := range snap.ModelsForProvider(p.ID) {
						if !m.Enabled {
							continue
						}
						egress := "cloud"
						for _, t := range m.ComplianceTags {
							if localTags[t] {
								egress = "local"
								break
							}
						}
						out = append(out, chatModelOption{
							ID:            m.ID,
							Tier:          string(m.Tier),
							Egress:        egress,
							OutUSDPerMTok: float64(m.Cost.OutputMicrosPerMillionToken) / 1e6,
						})
					}
				}
			}
		}
		sort.Slice(out, func(i, j int) bool {
			if tierOrder[out[i].Tier] != tierOrder[out[j].Tier] {
				return tierOrder[out[i].Tier] < tierOrder[out[j].Tier]
			}
			return out[i].ID < out[j].ID
		})
		if out == nil {
			out = []chatModelOption{}
		}
		_ = json.NewEncoder(w).Encode(out)
	}
}
