package generate

import (
	"sort"

	"github.com/nguyentin05/spectest/internal/domain"
)

// APIChange records a modification in an active API's hash or contract.
type APIChange struct {
	Key    string
	Before domain.APIState
	After  domain.API
}

// APIDiff categorizes operations into added, changed, removed, and restored sets.
type APIDiff struct {
	Added    []domain.API
	Changed  []APIChange
	Removed  []domain.APIStateRef
	Restored []domain.API
}

// IsEmpty reports whether no semantic changes were found in the API catalog.
func (d APIDiff) IsEmpty() bool {
	return len(d.Added) == 0 && len(d.Changed) == 0 && len(d.Removed) == 0 && len(d.Restored) == 0
}

// DiffCatalog compares the historical APICatalog against current SourceSet APIs.
func DiffCatalog(oldCatalog domain.APICatalog, currentSrc *domain.SourceSet) APIDiff {
	var diff APIDiff

	currentMap := make(map[string]domain.API, len(currentSrc.APIs))
	for _, a := range currentSrc.APIs {
		currentMap[a.Key] = a
	}

	// 1. Check current APIs against old catalog
	for _, a := range currentSrc.APIs {
		oldState, exists := oldCatalog[a.Key]
		if !exists {
			diff.Added = append(diff.Added, a)
			continue
		}

		if oldState.Status == domain.StatusRemoved {
			diff.Restored = append(diff.Restored, a)
			continue
		}

		if oldState.Hash != a.Hash {
			diff.Changed = append(diff.Changed, APIChange{
				Key:    a.Key,
				Before: oldState,
				After:  a,
			})
		}
	}

	// 2. Check old catalog for removed APIs
	for k, oldState := range oldCatalog {
		if oldState.Status == domain.StatusActive {
			if _, exists := currentMap[k]; !exists {
				diff.Removed = append(diff.Removed, domain.APIStateRef{
					Key:   k,
					State: oldState,
				})
			}
		}
	}

	// Sort all slices deterministically by API key
	sort.Slice(diff.Added, func(i, j int) bool { return diff.Added[i].Key < diff.Added[j].Key })
	sort.Slice(diff.Changed, func(i, j int) bool { return diff.Changed[i].Key < diff.Changed[j].Key })
	sort.Slice(diff.Removed, func(i, j int) bool { return diff.Removed[i].Key < diff.Removed[j].Key })
	sort.Slice(diff.Restored, func(i, j int) bool { return diff.Restored[i].Key < diff.Restored[j].Key })

	return diff
}
