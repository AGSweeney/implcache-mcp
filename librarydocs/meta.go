// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package librarydocs detects and parses LibraryDocs/ packages inside a
// repository checkout. Metadata is stored as a synthetic JSON document under
// the same ImplCache root (no schema migrations).
package librarydocs

import (
	"encoding/json"
	"path"
	"strings"
	"time"
)

// Handling modes for LibraryDocs during repository ingest.
const (
	HandlingAuto    = "auto"    // detect, parse, enrich, ranking signals
	HandlingNormal  = "normal"  // index files as ordinary docs
	HandlingExclude = "exclude" // skip LibraryDocs/ entirely
)

// Package states (progressive).
const (
	StateNotPresent   = "not_present"
	StateUnstructured = "unstructured"
	StateStructured   = "structured"
	StateValidated    = "validated"
	StateInvalid      = "invalid"
)

// Content classes for LibraryDocs paths.
const (
	ClassCuratedLibraryDoc  = "curated_library_doc"
	ClassCuratedProjectDoc  = "curated_project_doc"
	ClassCuratedPlatformDoc = "curated_platform_doc"
	ClassCuratedArtifact    = "curated_artifact"
	ClassIndex              = "librarydocs_index"
	ClassInventory          = "librarydocs_inventory"
	ClassValidation         = "librarydocs_validation"
	ClassLibraryDocsOther   = "librarydocs_other"
)

// TechnologyMeta marks the synthetic package metadata document.
const TechnologyMeta = "librarydocs-meta"

// MetaRelativePath is the repository-relative path used in the synthetic URI.
const MetaRelativePath = ".implcache/librarydocs-meta.json"

// MetaURI builds the well-known metadata document URI for a root.
func MetaURI(scheme, rootName string) string {
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		scheme = "git"
	}
	rootName = strings.Trim(rootName, "/")
	return scheme + "://" + rootName + "/" + MetaRelativePath
}

// NormalizeHandling returns a valid handling mode (default auto).
func NormalizeHandling(h string) string {
	switch strings.ToLower(strings.TrimSpace(h)) {
	case HandlingNormal:
		return HandlingNormal
	case HandlingExclude:
		return HandlingExclude
	default:
		return HandlingAuto
	}
}

// DocMeta is per-document LibraryDocs enrichment (stable keys for future tables).
type DocMeta struct {
	LibraryDocs       bool     `json:"librarydocs"`
	ContentClass      string   `json:"content_class,omitempty"`
	ComponentID       string   `json:"component_id,omitempty"`
	Component         string   `json:"component,omitempty"`
	Level             string   `json:"level,omitempty"`
	Reuse             string   `json:"reuse,omitempty"`
	Status            string   `json:"status,omitempty"`
	EvidenceLevel     string   `json:"evidence_level,omitempty"`
	Topics            []string `json:"topics,omitempty"`
	Platforms         []string `json:"platforms,omitempty"`
	SourcePaths       []string `json:"source_paths,omitempty"`
	RetrievalQuestions []string `json:"retrieval_questions,omitempty"`
	RelatedDocs       []string `json:"related_docs,omitempty"`
	ArtifactIDs       []string `json:"artifact_ids,omitempty"`
	Title             string   `json:"title,omitempty"`
	UnknownFrontmatter map[string]any `json:"unknown_frontmatter,omitempty"`
}

// ComponentRecord is one COMPONENT_INVENTORY.md row.
type ComponentRecord struct {
	ID              string   `json:"id"`
	Name            string   `json:"name,omitempty"`
	Level           string   `json:"level,omitempty"`
	Folder          string   `json:"folder,omitempty"`
	SourcePaths     []string `json:"source_paths,omitempty"`
	Reuse           string   `json:"reuse,omitempty"`
	Owner           string   `json:"owner,omitempty"`
	IOOwnership     string   `json:"io_ownership,omitempty"`
	ArtifactIDs     []string `json:"artifact_ids,omitempty"`
	DocStatus       string   `json:"doc_status,omitempty"`
	Evidence        string   `json:"evidence,omitempty"`
	Extra           map[string]string `json:"extra,omitempty"`
}

// IndexRecord is one INDEX.md row.
type IndexRecord struct {
	Path      string   `json:"path"`
	ID        string   `json:"id,omitempty"`
	Level     string   `json:"level,omitempty"`
	Component string   `json:"component,omitempty"`
	Purpose   string   `json:"purpose,omitempty"`
	Topics    []string `json:"topics,omitempty"`
	Status    string   `json:"status,omitempty"`
}

// ArtifactRecord is one artifacts/README.md row.
type ArtifactRecord struct {
	ID          string `json:"id"`
	File        string `json:"file,omitempty"`
	Component   string `json:"component,omitempty"`
	Usefulness  string `json:"usefulness,omitempty"`
	Description string `json:"description,omitempty"`
	ArtifactType string `json:"artifact_type,omitempty"`
}

// ValidationInfo is parsed from VALIDATION.md.
type ValidationInfo struct {
	Result          string `json:"result,omitempty"` // pass | fail
	Date            string `json:"date,omitempty"`
	Validator       string `json:"validator,omitempty"`
	StandardVersion string `json:"standard_version,omitempty"`
	Summary         string `json:"summary,omitempty"`
}

// PackageMeta is the canonical JSON stored in the synthetic metadata document.
type PackageMeta struct {
	Schema              string                    `json:"schema"`
	SchemaVersion       int                       `json:"schema_version"`
	LibraryDocs         bool                      `json:"librarydocs"`
	PackageState        string                    `json:"librarydocs_package_state"`
	Handling            string                    `json:"handling,omitempty"`
	StandardVersion     string                    `json:"standard_version,omitempty"`
	RootName            string                    `json:"root_name,omitempty"`
	ResolvedCommit      string                    `json:"resolved_commit,omitempty"`
	ImportedAt          string                    `json:"imported_at,omitempty"`
	Validation          *ValidationInfo           `json:"validation,omitempty"`
	Components          []ComponentRecord         `json:"components,omitempty"`
	Index               []IndexRecord             `json:"index,omitempty"`
	Artifacts           []ArtifactRecord          `json:"artifacts,omitempty"`
	Documents           map[string]DocMeta        `json:"documents,omitempty"`
	Warnings            []string                  `json:"warnings,omitempty"`
	Summary             PackageSummary            `json:"summary"`
}

// PackageSummary is the compact import/UI summary.
type PackageSummary struct {
	Detected               bool   `json:"detected"`
	PackageState           string `json:"packageState"`
	StandardVersion        string `json:"standardVersion,omitempty"`
	Libraries              int    `json:"libraries"`
	ProjectSubsystems      int    `json:"projectSubsystems"`
	PlatformConcerns       int    `json:"platformConcerns"`
	Artifacts              int    `json:"artifacts"`
	VerifiedDocuments      int    `json:"verifiedDocuments"`
	InferredDocuments      int    `json:"inferredDocuments"`
	DraftDocuments         int    `json:"draftDocuments"`
	PotentiallyStaleDocs   int    `json:"potentiallyStaleDocuments"`
	WarningCount           int    `json:"warnings"`
}

// NewPackageMeta returns an empty package meta shell.
func NewPackageMeta(rootName, handling string) *PackageMeta {
	return &PackageMeta{
		Schema:        "implcache.librarydocs",
		SchemaVersion: 1,
		LibraryDocs:   true,
		PackageState:  StateNotPresent,
		Handling:      NormalizeHandling(handling),
		RootName:      rootName,
		ImportedAt:    time.Now().UTC().Format(time.RFC3339),
		Documents:     map[string]DocMeta{},
		Summary:       PackageSummary{PackageState: StateNotPresent},
	}
}

// MarshalJSONBody returns pretty JSON for the synthetic document chunk.
func (m *PackageMeta) MarshalJSONBody() ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.MarshalIndent(m, "", "  ")
}

// ParsePackageMeta decodes a metadata document body.
func ParsePackageMeta(data []byte) (*PackageMeta, error) {
	var m PackageMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Documents == nil {
		m.Documents = map[string]DocMeta{}
	}
	return &m, nil
}

// DocMetaForPath returns metadata for a relative path, if any.
func (m *PackageMeta) DocMetaForPath(rel string) (DocMeta, bool) {
	if m == nil || m.Documents == nil {
		return DocMeta{}, false
	}
	rel = path.Clean("/" + filepathToSlash(rel))
	rel = strings.TrimPrefix(rel, "/")
	d, ok := m.Documents[rel]
	return d, ok
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, `\`, `/`)
}

// RecomputeSummary fills Summary counts from parsed records and documents.
func (m *PackageMeta) RecomputeSummary() {
	if m == nil {
		return
	}
	s := PackageSummary{
		Detected:     m.PackageState != StateNotPresent,
		PackageState: m.PackageState,
		StandardVersion: m.StandardVersion,
		WarningCount: len(m.Warnings),
		Artifacts:    len(m.Artifacts),
	}
	for _, c := range m.Components {
		switch strings.ToLower(c.Level) {
		case "library":
			s.Libraries++
		case "project":
			s.ProjectSubsystems++
		case "platform":
			s.PlatformConcerns++
		default:
			// infer from ID prefix
			id := strings.ToUpper(c.ID)
			switch {
			case strings.HasPrefix(id, "L"):
				s.Libraries++
			case strings.HasPrefix(id, "P") && !strings.HasPrefix(id, "PL"):
				s.ProjectSubsystems++
			case strings.HasPrefix(id, "PL"):
				s.PlatformConcerns++
			}
		}
	}
	for _, d := range m.Documents {
		switch strings.ToLower(d.Status) {
		case "verified":
			s.VerifiedDocuments++
		case "inferred":
			s.InferredDocuments++
		case "draft", "experimental":
			s.DraftDocuments++
		}
	}
	if m.Validation != nil && m.Validation.StandardVersion != "" && s.StandardVersion == "" {
		s.StandardVersion = m.Validation.StandardVersion
	}
	m.Summary = s
}
