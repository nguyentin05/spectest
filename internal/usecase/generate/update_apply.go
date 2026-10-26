package generate

import (
	"github.com/nguyentin05/spectest/internal/domain"
)


// buildCatalogPatch creates the delta patch of APIState entries for an update.
func buildCatalogPatch(diff APIDiff) domain.APICatalog {
	patch := make(domain.APICatalog)

	for _, a := range diff.Added {
		patch[a.Key] = domain.APIState{
			Status: domain.StatusActive,
			Hash:   a.Hash,
			Sig:    a.Sig,
		}
	}
	for _, ch := range diff.Changed {
		patch[ch.Key] = domain.APIState{
			Status: domain.StatusActive,
			Hash:   ch.After.Hash,
			Sig:    ch.After.Sig,
		}
	}
	for _, rest := range diff.Restored {
		patch[rest.Key] = domain.APIState{
			Status: domain.StatusActive,
			Hash:   rest.Hash,
			Sig:    rest.Sig,
		}
	}
	for _, rem := range diff.Removed {
		patch[rem.Key] = domain.APIState{
			Status: domain.StatusRemoved,
			Hash:   rem.State.Hash,
			Sig:    rem.State.Sig,
		}
	}

	return patch
}

// buildOpenAPITrigger compiles the list of changed API keys for the update log trigger.
func buildOpenAPITrigger(diff APIDiff) *domain.OpenAPITrigger {
	var trigger domain.OpenAPITrigger

	for _, a := range diff.Added {
		trigger.Added = append(trigger.Added, a.Key)
	}
	for _, ch := range diff.Changed {
		trigger.Changed = append(trigger.Changed, ch.Key)
	}
	for _, rem := range diff.Removed {
		trigger.Removed = append(trigger.Removed, rem.Key)
	}
	for _, rest := range diff.Restored {
		trigger.Restored = append(trigger.Restored, rest.Key)
	}

	return &trigger
}
