// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"context"
	"os"
	"strconv"
	"strings"

	"implcache-mcp/store"
)

// RankingConfig holds configurable LibraryDocs score additives (not unexplained constants).
type RankingConfig struct {
	Enabled              bool
	VerifiedE1E2Boost    float64
	ValidatedPackageBoost float64
	DraftPenalty         float64
	InferredPenalty      float64
	DeprecatedPenalty    float64
	InvalidPackagePenalty float64
	IndexInventoryPenalty float64 // INDEX/inventory/validation are not primary answers
}

// DefaultRankingConfig reads optional IMPLCACHE_LIBRARYDOCS_* env overrides.
func DefaultRankingConfig() RankingConfig {
	c := RankingConfig{
		Enabled:               envBool("IMPLCACHE_LIBRARYDOCS_RANKING", true),
		VerifiedE1E2Boost:     envFloat("IMPLCACHE_LIBRARYDOCS_BOOST_VERIFIED", 6),
		ValidatedPackageBoost: envFloat("IMPLCACHE_LIBRARYDOCS_BOOST_VALIDATED", 2),
		DraftPenalty:          envFloat("IMPLCACHE_LIBRARYDOCS_PENALTY_DRAFT", 8),
		InferredPenalty:       envFloat("IMPLCACHE_LIBRARYDOCS_PENALTY_INFERRED", 3),
		DeprecatedPenalty:     envFloat("IMPLCACHE_LIBRARYDOCS_PENALTY_DEPRECATED", 12),
		InvalidPackagePenalty: envFloat("IMPLCACHE_LIBRARYDOCS_PENALTY_INVALID", 10),
		IndexInventoryPenalty: envFloat("IMPLCACHE_LIBRARYDOCS_PENALTY_INDEX", 15),
	}
	return c
}

func envBool(k string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envFloat(k string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// HitMeta is attached to SearchHit.LibraryDocs for Search Lab / API.
type HitMeta struct {
	ContentClass        string   `json:"contentClass,omitempty"`
	ComponentID         string   `json:"componentId,omitempty"`
	Component           string   `json:"component,omitempty"`
	Level               string   `json:"level,omitempty"`
	Status              string   `json:"status,omitempty"`
	Evidence            string   `json:"evidence,omitempty"`
	PackageState        string   `json:"packageState,omitempty"`
	SourcePaths         []string `json:"sourcePaths,omitempty"`
	ArtifactIDs         []string `json:"artifactIds,omitempty"`
	RankingContribution float64  `json:"rankingContribution,omitempty"`
}

// EnrichHits attaches LibraryDocs metadata and applies configurable score deltas.
// Exact symbol matchKind is never overridden downward in priority: boosts are additive only.
func EnrichHits(ctx context.Context, st *store.Store, hits []store.SearchHit, cfg RankingConfig) []store.SearchHit {
	if st == nil || len(hits) == 0 || !cfg.Enabled {
		return hits
	}
	cache := map[string]*PackageMeta{}
	for i := range hits {
		h := &hits[i]
		root := h.RootName
		if root == "" {
			continue
		}
		meta, ok := cache[root]
		if !ok {
			meta, _ = LoadMeta(ctx, st, schemeFromURI(h.URI), root)
			if meta == nil {
				// try alternate scheme
				alt := "project"
				if schemeFromURI(h.URI) == "project" {
					alt = "git"
				}
				meta, _ = LoadMeta(ctx, st, alt, root)
			}
			cache[root] = meta
		}
		if meta == nil || meta.Handling == HandlingNormal {
			continue
		}
		rel := h.Path
		if rel == "" {
			rel = pathFromURI(h.URI)
		}
		dm, found := meta.DocMetaForPath(rel)
		if !found && !IsLibraryDocsPath(rel) {
			continue
		}
		if !found {
			dm = DocMeta{LibraryDocs: true, ContentClass: ClassifyPath(rel)}
		}
		contrib := scoreContribution(cfg, meta.PackageState, dm)
		hm := &HitMeta{
			ContentClass:        dm.ContentClass,
			ComponentID:         dm.ComponentID,
			Component:           dm.Component,
			Level:               dm.Level,
			Status:              dm.Status,
			Evidence:            dm.EvidenceLevel,
			PackageState:        meta.PackageState,
			SourcePaths:         dm.SourcePaths,
			ArtifactIDs:         dm.ArtifactIDs,
			RankingContribution: contrib,
		}
		h.LibraryDocs = hm
		if h.ContentClass == "" {
			h.ContentClass = dm.ContentClass
		}
		if contrib != 0 {
			h.Score += contrib
			if h.ScoreBreakdown != nil {
				h.ScoreBreakdown.LibraryDocsBoost = contrib
				h.ScoreBreakdown.Total += contrib
			}
		}
	}
	return hits
}

func scoreContribution(cfg RankingConfig, pkgState string, dm DocMeta) float64 {
	var c float64
	switch dm.ContentClass {
	case ClassIndex, ClassInventory, ClassValidation:
		c -= cfg.IndexInventoryPenalty
	}
	if pkgState == StateInvalid {
		c -= cfg.InvalidPackagePenalty
	}
	if pkgState == StateValidated {
		c += cfg.ValidatedPackageBoost
	}
	status := strings.ToLower(dm.Status)
	ev := strings.ToUpper(dm.EvidenceLevel)
	if status == "verified" && (ev == "E1" || ev == "E2") {
		c += cfg.VerifiedE1E2Boost
	}
	if status == "inferred" || ev == "E3" {
		c -= cfg.InferredPenalty
	}
	if status == "draft" || status == "experimental" || ev == "E4" {
		c -= cfg.DraftPenalty
	}
	if status == "deprecated" {
		c -= cfg.DeprecatedPenalty
	}
	return c
}

func schemeFromURI(uri string) string {
	i := strings.Index(uri, "://")
	if i <= 0 {
		return "git"
	}
	return uri[:i]
}

func pathFromURI(uri string) string {
	i := strings.Index(uri, "://")
	if i < 0 {
		return uri
	}
	rest := uri[i+3:]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return ""
	}
	return rest[slash+1:]
}

// FilterHits applies Search Lab LibraryDocs filters in the API layer.
func FilterHits(hits []store.SearchHit, libraryDocsOnly, excludeLibraryDocs bool, level, status string) []store.SearchHit {
	if !libraryDocsOnly && !excludeLibraryDocs && level == "" && status == "" {
		return hits
	}
	level = strings.ToLower(strings.TrimSpace(level))
	status = strings.ToLower(strings.TrimSpace(status))
	out := make([]store.SearchHit, 0, len(hits))
	for _, h := range hits {
		isLD := h.LibraryDocs != nil || IsLibraryDocsPath(h.Path) || strings.HasPrefix(strings.ToLower(h.ContentClass), "curated_") || strings.HasPrefix(h.ContentClass, "librarydocs_")
		if libraryDocsOnly && !isLD {
			continue
		}
		if excludeLibraryDocs && isLD {
			continue
		}
		if level != "" {
			hm, _ := h.LibraryDocs.(*HitMeta)
			if hm == nil || strings.ToLower(hm.Level) != level {
				continue
			}
		}
		if status != "" {
			hm, _ := h.LibraryDocs.(*HitMeta)
			if hm == nil || strings.ToLower(hm.Status) != status {
				continue
			}
		}
		out = append(out, h)
	}
	return out
}
