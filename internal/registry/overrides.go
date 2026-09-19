package registry

// ApplyProviderModelOverrides sets the ProviderModelID (the concrete model sent
// to the provider, e.g. "z-ai/glm-4.6") of each tier's model from the overrides
// map, keyed by Tier. Empty or missing values leave the registry default. It
// lets a deployment swap which provider model fills each tier — cheap / balanced
// / premium — without changing code.
//
// Note: only the model slug is overridden, not its pricing metadata. Realized
// cost is taken from provider usage at request time and stays accurate, but
// pre-call cost estimates and the dashboard's all-premium savings baseline use
// the registry's pricing for that tier until that is updated too.
func ApplyProviderModelOverrides(def Definition, overrides map[Tier]string) Definition {
	for i := range def.Models {
		if slug, ok := overrides[def.Models[i].Tier]; ok {
			if slug != "" {
				def.Models[i].ProviderModelID = slug
			}
		}
	}
	return def
}
