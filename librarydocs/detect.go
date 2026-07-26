// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectSignals are files/dirs used for progressive detection.
var DetectSignals = []string{
	"LibraryDocs/README.md",
	"LibraryDocs/INDEX.md",
	"LibraryDocs/project/COMPONENT_INVENTORY.md",
	"LibraryDocs/artifacts/README.md",
	"LibraryDocs/VALIDATION.md",
	"LibraryDocs/libraries",
	"LibraryDocs/project",
	"LibraryDocs/platform",
}

// AnalyzeCheckout detects and parses LibraryDocs under checkout.
// Never returns a fatal error for malformed docs; warnings are in PackageMeta.
func AnalyzeCheckout(checkout, rootName, handling, resolvedCommit string) *PackageMeta {
	meta := NewPackageMeta(rootName, handling)
	meta.ResolvedCommit = resolvedCommit
	handling = NormalizeHandling(handling)
	meta.Handling = handling

	root := LibraryDocsRoot(checkout)
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		meta.PackageState = StateNotPresent
		meta.LibraryDocs = false
		meta.RecomputeSummary()
		return meta
	}

	if handling == HandlingExclude {
		meta.PackageState = StateNotPresent
		meta.LibraryDocs = false
		meta.Warnings = append(meta.Warnings, "LibraryDocs handling=exclude; directory skipped")
		meta.RecomputeSummary()
		return meta
	}

	hasIndex := fileExists(filepath.Join(checkout, filepath.FromSlash(IndexRelativePath)))
	hasInv := fileExists(filepath.Join(checkout, filepath.FromSlash(InventoryRelativePath)))

	if handling == HandlingNormal {
		meta.PackageState = StateUnstructured
		if hasIndex && hasInv {
			meta.PackageState = StateStructured
		}
		meta.Warnings = append(meta.Warnings, "LibraryDocs handling=normal; structure noted without enrichment")
		meta.RecomputeSummary()
		return meta
	}

	// auto
	if !hasIndex || !hasInv {
		meta.PackageState = StateUnstructured
		if !hasIndex {
			meta.Warnings = append(meta.Warnings, "LibraryDocs present but missing INDEX.md")
		}
		if !hasInv {
			meta.Warnings = append(meta.Warnings, "LibraryDocs present but missing project/COMPONENT_INVENTORY.md")
		}
		// still try optional parsers / frontmatter walk for whatever exists
		enrichFromTree(checkout, meta)
		meta.RecomputeSummary()
		return meta
	}

	meta.PackageState = StateStructured
	enrichFromTree(checkout, meta)

	// validation
	if vi, warns, err := ParseValidationFile(checkout); err == nil && vi != nil {
		meta.Validation = vi
		meta.Warnings = append(meta.Warnings, warns...)
		if vi.StandardVersion != "" {
			meta.StandardVersion = vi.StandardVersion
		}
		switch vi.Result {
		case "pass":
			// only validated if structure still present (we have index+inv)
			meta.PackageState = StateValidated
		case "fail":
			meta.PackageState = StateInvalid
			meta.Warnings = append(meta.Warnings, ValidationRelativePath+" reports fail")
		}
	}

	// malformed structured package: empty inventory after parse
	if len(meta.Components) == 0 {
		meta.PackageState = StateInvalid
		meta.Warnings = append(meta.Warnings, InventoryRelativePath+": no component rows parsed")
	}

	meta.RecomputeSummary()
	return meta
}

func enrichFromTree(checkout string, meta *PackageMeta) {
	if inv, warns, err := ParseInventoryFile(checkout); err == nil {
		meta.Components = inv
		meta.Warnings = append(meta.Warnings, warns...)
	} else if !os.IsNotExist(err) {
		meta.Warnings = append(meta.Warnings, InventoryRelativePath+": "+err.Error())
	}

	if idx, warns, err := ParseIndexFile(checkout); err == nil {
		meta.Index = idx
		meta.Warnings = append(meta.Warnings, warns...)
	} else if !os.IsNotExist(err) {
		meta.Warnings = append(meta.Warnings, IndexRelativePath+": "+err.Error())
	}

	if arts, warns, err := ParseArtifactsFile(checkout); err == nil {
		meta.Artifacts = arts
		meta.Warnings = append(meta.Warnings, warns...)
	}

	meta.Warnings = append(meta.Warnings, CrossCheckIndexInventory(meta.Index, meta.Components)...)
	meta.Warnings = append(meta.Warnings, CheckArtifactRefs(meta.Components, meta.Artifacts)...)

	// Walk LibraryDocs markdown for frontmatter / path classification
	_ = filepath.WalkDir(LibraryDocsRoot(checkout), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(checkout, p)
		if err != nil {
			return nil
		}
		rel = filepathToSlash(rel)
		class := ClassifyPath(rel)
		if class == "" {
			return nil
		}
		dm := DocMeta{
			LibraryDocs:  true,
			ContentClass: class,
		}
		// merge inventory/index hints
		for _, c := range meta.Components {
			folder := filepathToSlash(c.Folder)
			if folder != "" && (rel == folder || strings.HasPrefix(rel, strings.TrimSuffix(folder, "/")+"/")) {
				dm.ComponentID = c.ID
				if dm.Component == "" {
					dm.Component = c.Name
				}
				if dm.Level == "" {
					dm.Level = c.Level
				}
				if dm.Reuse == "" {
					dm.Reuse = c.Reuse
				}
				if dm.Status == "" {
					dm.Status = c.DocStatus
				}
				if dm.EvidenceLevel == "" {
					dm.EvidenceLevel = c.Evidence
				}
				if len(dm.SourcePaths) == 0 {
					dm.SourcePaths = c.SourcePaths
				}
				if len(dm.ArtifactIDs) == 0 {
					dm.ArtifactIDs = c.ArtifactIDs
				}
			}
		}
		for _, ix := range meta.Index {
			if ix.Path == rel {
				if dm.ComponentID == "" {
					dm.ComponentID = ix.ID
				}
				if dm.Component == "" {
					dm.Component = ix.Component
				}
				if dm.Level == "" {
					dm.Level = ix.Level
				}
				if dm.Status == "" {
					dm.Status = ix.Status
				}
				if len(dm.Topics) == 0 {
					dm.Topics = ix.Topics
				}
			}
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".md" || ext == ".markdown" {
			data, err := os.ReadFile(p)
			if err == nil {
				fm, _, warn := SplitFrontmatter(string(data))
				if warn != "" {
					meta.Warnings = append(meta.Warnings, rel+": "+warn)
				} else {
					var pw []string
					ApplyFrontmatter(&dm, fm, &pw)
					meta.Warnings = append(meta.Warnings, prefixPathWarns(rel, pw)...)
				}
			}
		}
		if class == ClassCuratedArtifact {
			for _, a := range meta.Artifacts {
				if a.File == rel {
					dm.ArtifactIDs = appendUnique(dm.ArtifactIDs, a.ID)
					if dm.ComponentID == "" {
						dm.ComponentID = a.Component
					}
				}
			}
		}
		meta.Documents[rel] = dm
		return nil
	})
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func appendUnique(slice []string, v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return slice
	}
	for _, s := range slice {
		if s == v {
			return slice
		}
	}
	return append(slice, v)
}
