package openapi

import (
	"strings"
)

// DomainChunk represents a cohesive cluster of API endpoints grouped by domain tag or path prefix.
type DomainChunk struct {
	Tag           string                `json:"tag"`
	TokenProducer *NormalizedOperation  `json:"token_producer,omitempty"`
	Operations    []NormalizedOperation `json:"operations"`
	PRDSection    string                `json:"prd_section,omitempty"`
}

// ChunkByDomain partitions normalized operations by OpenAPI tag (or path prefix) into chunks
// of at most maxPerChunk endpoints, attaching any detected token producer to consumer chunks.
func ChunkByDomain(ops []NormalizedOperation, maxPerChunk int) []DomainChunk {
	if maxPerChunk <= 0 {
		maxPerChunk = 5
	}

	var producer *NormalizedOperation
	for i := range ops {
		if ops[i].IsTokenProducer {
			p := ops[i]
			producer = &p
			break
		}
	}

	tagOrder := []string{}
	grouped := make(map[string][]NormalizedOperation)

	for _, op := range ops {
		tag := primaryTag(op)
		if _, exists := grouped[tag]; !exists {
			tagOrder = append(tagOrder, tag)
		}
		grouped[tag] = append(grouped[tag], op)
	}

	var chunks []DomainChunk
	for _, tag := range tagOrder {
		groupOps := grouped[tag]
		for i := 0; i < len(groupOps); i += maxPerChunk {
			end := i + maxPerChunk
			if end > len(groupOps) {
				end = len(groupOps)
			}
			slice := groupOps[i:end]
			chunk := DomainChunk{
				Tag:        tag,
				Operations: slice,
			}
			needsProducer := false
			hasProducerInside := false
			for _, op := range slice {
				if op.RequiresAuth {
					needsProducer = true
				}
				if op.IsTokenProducer {
					hasProducerInside = true
				}
			}
			if needsProducer && !hasProducerInside && producer != nil {
				chunk.TokenProducer = producer
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

func primaryTag(op NormalizedOperation) string {
	if len(op.Tags) > 0 && strings.TrimSpace(op.Tags[0]) != "" {
		return strings.TrimSpace(op.Tags[0])
	}
	parts := strings.Split(strings.Trim(op.Endpoint, "/"), "/")
	for _, p := range parts {
		if p != "" && p != "api" && p != "v1" && p != "v2" && !strings.HasPrefix(p, "{") {
			return p
		}
	}
	return "default"
}
