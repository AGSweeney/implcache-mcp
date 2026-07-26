// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import "time"

// Result status values (PRD §8.3).
const (
	StatusGroundedCurated       = "grounded_curated"
	StatusGroundedLocal         = "grounded_local"
	StatusGroundedMixed         = "grounded_mixed"
	StatusRootSelectionRequired = "root_selection_required"
	StatusLocalInsufficient     = "local_insufficient"
	StatusNoLocalMatch          = "no_local_match"
	StatusRequestError          = "request_error"
)

// Coverage values.
const (
	CoverageHigh          = "high"
	CoverageMedium        = "medium"
	CoverageLow           = "low"
	CoverageUnclassified  = "unclassified"
	CoverageNotApplicable = "not_applicable"
)

// Request class values.
const (
	ClassImplementationContext = "implementation_context"
	ClassKnowledgeSearch       = "knowledge_search"
	ClassSymbolSearch          = "symbol_search"
	ClassDocumentFetch         = "document_fetch"
	ClassRootResolution        = "root_resolution"
	ClassOutcomeReport         = "outcome_report"
	ClassOther                 = "other"
)

// Evidence types.
const (
	EvidenceCitation = "citation"
	EvidenceSymbol   = "symbol"
	EvidenceRecipe   = "recipe"
	EvidenceCurated  = "curated"
	EvidenceDocument = "document"
)

// Config controls analytics behaviour (persisted partially in usage_meta).
type Config struct {
	Enabled           bool   `json:"enabled"`
	DBPath            string `json:"dbPath"`
	RetentionDays     int    `json:"retentionDays"` // 0 = unlimited
	StoreTaskText     bool   `json:"storeTaskText"`
	StoreEvidenceText bool   `json:"storeEvidenceText"`
	CLIDisabled       bool   `json:"cliDisabled,omitempty"` // -telemetry=off wins
}

// Status is the public analytics status payload.
type Status struct {
	Enabled               bool   `json:"enabled"`
	Available             bool   `json:"available"`
	DBPath                string `json:"dbPath,omitempty"`
	RetentionDays         int    `json:"retentionDays"`
	StoreTaskText         bool   `json:"storeTaskText"`
	StoreEvidenceText     bool   `json:"storeEvidenceText"`
	DatabaseBytes         int64  `json:"databaseBytes,omitempty"`
	OldestAt              string `json:"oldestAt,omitempty"`
	NewestAt              string `json:"newestAt,omitempty"`
	RequestCount          int64  `json:"requestCount"`
	DroppedEvents         int64  `json:"droppedEvents"`
	Message               string `json:"message,omitempty"`
	LocalOnly             bool   `json:"localOnly"`
	MetadataOnly          bool   `json:"metadataOnly"`
	TokenEstimatorVersion string `json:"tokenEstimatorVersion,omitempty"`
	SchemaVersion         int    `json:"schemaVersion,omitempty"`
}

// RequestEvent is one retrieval request telemetry record.
type RequestEvent struct {
	RequestID                      string
	OccurredAt                     time.Time
	SessionHash                    string
	ClientName                     string
	ModelName                      string
	ToolName                       string
	TaskHash                       string
	TaskSummary                    string // only when StoreTaskText
	ResultStatus                   string
	Coverage                       string
	Freshness                      string
	LatencyMS                      int
	EstimatedTokens                int // returned package tokens (legacy alias)
	ReturnedTokens                 int
	StructuredTokens               int
	RawDocumentTokens              int
	EstimatedSource                int
	TokensAvoided                  int
	ReductionPct                   *float64
	TokenEstimatorVersion          string
	CoverageApplicable             *bool
	RequestClass                   string
	ContextFingerprint             string
	RootSelectionRequired          bool
	AdditionalRetrievalRecommended bool
	RootCount                      int
	SourceCount                    int
	CitationCount                  int
	CuratedCount                   int
	RecipeCount                    int
	SymbolCount                    int
	ErrorCategory                  string
	ErrorMessage                   string
	Roots                          []RootRef
	Evidence                       []EvidenceEvent
}

// RootRef is a root associated with a request.
type RootRef struct {
	RootKey      string
	RootName     string
	RootGroupKey string
	RootRole     string
	Selected     bool
}

// EvidenceEvent is metadata for one piece of evidence (no body by default).
type EvidenceEvent struct {
	EvidenceType          string `json:"evidenceType"`
	EvidenceKey           string `json:"evidenceKey,omitempty"`
	RootKey               string `json:"rootKey,omitempty"`
	SourceURI             string `json:"sourceUri,omitempty"`
	Authority             string `json:"authority,omitempty"`
	RankPosition          int    `json:"rankPosition"`
	SelectedForPackage    bool   `json:"selectedForPackage"`
	IncludedAfterTrimming bool   `json:"includedAfterTrimming"`
	EstimatedTokens       int    `json:"estimatedTokens,omitempty"`
	SourceHash            string `json:"sourceHash,omitempty"`
}
