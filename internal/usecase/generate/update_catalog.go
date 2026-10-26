package generate

import (
	"errors"
	"fmt"

	"github.com/nguyentin05/spectest/internal/domain"
)

// ReconstructCatalog replays the update log's CatalogPatch entries onto the
// base APICatalog to produce the active/removed catalog state at HEAD.
func ReconstructCatalog(log *domain.LogFile) (domain.APICatalog, error) {
	if log == nil || log.Base.APICatalog == nil {
		return nil, errors.New("history log has no API catalog; regenerate once with the current format")
	}

	catalog := make(domain.APICatalog, len(log.Base.APICatalog))
	for k, v := range log.Base.APICatalog {
		catalog[k] = v
	}

	for i, u := range log.Update {
		expectedN := i + 1
		if u.N != expectedN {
			return nil, fmt.Errorf("invalid update sequence: update at index %d has N=%d, want N=%d", i, u.N, expectedN)
		}

		for k, patchState := range u.CatalogPatch {
			catalog[k] = patchState
		}
	}

	return catalog, nil
}
