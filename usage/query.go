// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Filter constrains analytics queries.
type Filter struct {
	From         time.Time
	To           time.Time
	Root         string
	Tool         string
	Coverage     string
	Status       string
	RequestClass string
	Bucket       string // hour|day|week|month
	Limit        int
	Offset       int
	Sort         string
	Order        string // asc|desc
}

// Summary is Overview card metrics.
type Summary struct {
	TotalRequests               int64    `json:"totalRequests"`
	GroundedRequests            int64    `json:"groundedRequests"`
	LocalEvidenceRate           float64  `json:"localEvidenceRate"`
	CuratedRequests             int64    `json:"curatedRequests"`
	CuratedUsageRate            float64  `json:"curatedUsageRate"`
	HighCoverage                int64    `json:"highCoverage"`
	MediumCoverage              int64    `json:"mediumCoverage"`
	LowCoverage                 int64    `json:"lowCoverage"`
	UnclassifiedCoverage        int64    `json:"unclassifiedCoverage"`
	NotApplicableCoverage       int64    `json:"notApplicableCoverage"`
	HighCoverageRate            float64  `json:"highCoverageRate"`
	MediumCoverageRate          float64  `json:"mediumCoverageRate"`
	LowCoverageRate             float64  `json:"lowCoverageRate"`
	UnclassifiedCoverageRate    float64  `json:"unclassifiedCoverageRate"`
	RootSelectionRequired       int64    `json:"rootSelectionRequired"`
	RootSelectionRate           float64  `json:"rootSelectionRate"`
	LocalContextTokensServed    int64    `json:"localContextTokensServed"`
	AvgPackageTokens            *float64 `json:"avgPackageTokens,omitempty"`
	NoLocalMatch                int64    `json:"noLocalMatch"`
	LocalInsufficient           int64    `json:"localInsufficient"`
	Errors                      int64    `json:"errors"`
	ReconcileOK                 bool     `json:"reconcileOk"`
	ReconcileSum                int64    `json:"reconcileSum"`
	UnclassifiedCoverageWarning bool     `json:"unclassifiedCoverageWarning"`
	TokenEstimatorVersion       string   `json:"tokenEstimatorVersion,omitempty"`
}

// TimePoint is one bucket in a timeseries.
type TimePoint struct {
	Bucket       string `json:"bucket"`
	Total        int64  `json:"total"`
	Grounded     int64  `json:"grounded"`
	RootChoice   int64  `json:"rootChoice"`
	NoMatch      int64  `json:"noMatch"`
	Insufficient int64  `json:"insufficient"`
	Errors       int64  `json:"errors"`
	High         int64  `json:"high,omitempty"`
	Medium       int64  `json:"medium,omitempty"`
	Low          int64  `json:"low,omitempty"`
	TokensServed int64  `json:"tokensServed,omitempty"`
	SourceTokens int64  `json:"sourceTokens,omitempty"`
	TokensAvoided int64 `json:"tokensAvoided,omitempty"`
	AvgPackage   *float64 `json:"avgPackage,omitempty"`
	AvgReduction *float64 `json:"avgReduction,omitempty"`
}

// CoverageBreakdown is an explicit coverage/status distribution (not a timeseries).
type CoverageBreakdown struct {
	High                  int64 `json:"high"`
	Medium                int64 `json:"medium"`
	Low                   int64 `json:"low"`
	Unclassified          int64 `json:"unclassified"`
	NotApplicable         int64 `json:"notApplicable"`
	LocalInsufficient     int64 `json:"localInsufficient"`
	NoLocalMatch          int64 `json:"noLocalMatch"`
	RootSelectionRequired int64 `json:"rootSelectionRequired"`
	Errors                int64 `json:"errors"`
	Grounded              int64 `json:"grounded"`
	Total                 int64 `json:"total"`
	UnclassifiedWarning   bool  `json:"unclassifiedWarning"`
}

// OutcomeBreakdown is mutually exclusive per-request classification.
type OutcomeBreakdown struct {
	Curated      int64 `json:"curated"`
	Local        int64 `json:"local"`
	Mixed        int64 `json:"mixed"`
	Recipe       int64 `json:"recipe"`
	SymbolLed    int64 `json:"symbolLed"`
	RawDocLed    int64 `json:"rawDocLed"`
	NoMatch      int64 `json:"noMatch"`
	Insufficient int64 `json:"insufficient"`
	RootChoice   int64 `json:"rootSelectionRequired"`
	Errors       int64 `json:"errors"`
	Other        int64 `json:"other"`
	Total        int64 `json:"total"`
}

// EvidenceUsage counts requests that used each evidence type (overlapping).
type EvidenceUsage struct {
	Curated      int64 `json:"curated"`
	Recipe       int64 `json:"recipe"`
	Symbol       int64 `json:"symbol"`
	RawDocuments int64 `json:"rawDocuments"`
	Document     int64 `json:"document"`
}

// GroundingReport splits exclusive outcomes from overlapping evidence usage.
type GroundingReport struct {
	Outcomes OutcomeBreakdown `json:"outcomes"`
	Evidence EvidenceUsage    `json:"evidence"`
}

// EfficiencyReport is the Efficiency tab payload.
type EfficiencyReport struct {
	LocalContextTokensServed   *int64             `json:"localContextTokensServed"`
	StructuredTokensServed     *int64             `json:"structuredTokensServed"`
	RawDocumentTokensServed    *int64             `json:"rawDocumentTokensServed"`
	AvgPackageTokens           *float64           `json:"avgPackageTokens"`
	EstimatedSourceTokens      *int64             `json:"estimatedSourceTokens"`
	EstimatedTokensAvoided     *int64             `json:"estimatedTokensAvoided"`
	AvgContextReductionPercent *float64           `json:"avgContextReductionPercent"`
	RawDocumentShare           *float64           `json:"rawDocumentShare"`
	TokensPerGroundedRequest   *float64           `json:"tokensPerGroundedRequest"`
	TokensPerSuccessfulOutcome *float64           `json:"tokensPerSuccessfulOutcome"`
	TokenEstimatorVersion      string             `json:"tokenEstimatorVersion"`
	SourceTypeBreakdown        []TokenTypeBucket  `json:"sourceTypeBreakdown"`
	TokenTimeseries            []TimePoint        `json:"tokenTimeseries"`
	PackageTimeseries          []TimePoint        `json:"packageTimeseries"`
	GroundedRequests           int64              `json:"groundedRequests"`
	SuccessfulOutcomes         int64              `json:"successfulOutcomes"`
}

// TokenTypeBucket is tokens attributed to an evidence type.
type TokenTypeBucket struct {
	Type   string `json:"type"`
	Tokens int64  `json:"tokens"`
	Label  string `json:"label"`
}

// KnowledgeReport ranks useful knowledge entities from usage.
type KnowledgeReport struct {
	Roots    []KnowledgeRank `json:"roots"`
	Evidence []KnowledgeRank `json:"evidence"`
	Curated  []KnowledgeRank `json:"curated"`
}

// KnowledgeRank is one ranked knowledge key.
type KnowledgeRank struct {
	Key          string  `json:"key"`
	Label        string  `json:"label,omitempty"`
	TimesSelected int64  `json:"timesSelected"`
	TimesIncluded int64  `json:"timesIncluded"`
	AvgCoverage  *float64 `json:"avgCoverage,omitempty"`
}

// RequestRow is a recent-request list entry.
type RequestRow struct {
	RequestID       string   `json:"requestId"`
	OccurredAt      string   `json:"occurredAt"`
	ToolName        string   `json:"toolName"`
	ResultStatus    string   `json:"resultStatus"`
	Coverage        string   `json:"coverage,omitempty"`
	EstimatedTokens int      `json:"estimatedTokens"`
	ReturnedTokens  int      `json:"returnedTokens"`
	SourceCount     int      `json:"sourceCount"`
	LatencyMS       int      `json:"latencyMs"`
	RequestClass    string   `json:"requestClass,omitempty"`
	Roots           []string `json:"roots,omitempty"`
	Curated         bool     `json:"curated"`
}

// RequestDetail is a drill-down payload.
type RequestDetail struct {
	RequestRow
	ClientName                     string          `json:"clientName,omitempty"`
	ModelName                      string          `json:"modelName,omitempty"`
	SessionHash                    string          `json:"sessionHash,omitempty"`
	Freshness                      string          `json:"freshness,omitempty"`
	ContextFingerprint             string          `json:"contextFingerprint,omitempty"`
	TaskHash                       string          `json:"taskHash,omitempty"`
	RootSelectionRequired          bool            `json:"rootSelectionRequired"`
	AdditionalRetrievalRecommended bool            `json:"additionalRetrievalRecommended"`
	CitationCount                  int             `json:"citationCount"`
	CuratedCount                   int             `json:"curatedCount"`
	RecipeCount                    int             `json:"recipeCount"`
	SymbolCount                    int             `json:"symbolCount"`
	StructuredTokens               int             `json:"structuredTokens"`
	RawDocumentTokens              int             `json:"rawDocumentTokens"`
	EstimatedSourceTokens          int             `json:"estimatedSourceTokens"`
	EstimatedTokensAvoided         int             `json:"estimatedTokensAvoided"`
	ContextReductionPercent        *float64        `json:"contextReductionPercent,omitempty"`
	TokenEstimatorVersion          string          `json:"tokenEstimatorVersion,omitempty"`
	CoverageApplicable             *bool           `json:"coverageApplicable,omitempty"`
	ErrorCategory                  string          `json:"errorCategory,omitempty"`
	ErrorMessage                   string          `json:"errorMessage,omitempty"`
	Evidence                       []EvidenceEvent `json:"evidence,omitempty"`
}

// RequestList is a paginated request list.
type RequestList struct {
	Requests []RequestRow `json:"requests"`
	Count    int          `json:"count"`
	Total    int64        `json:"total"`
	Limit    int          `json:"limit"`
	Offset   int          `json:"offset"`
}

const returnedTokensSQL = `COALESCE(returned_tokens, estimated_tokens, 0)`

func (s *Store) whereFilter(f Filter) (string, []any) {
	var parts []string
	var args []any
	if !f.From.IsZero() {
		parts = append(parts, `occurred_at >= ?`)
		args = append(args, f.From.UTC().Format(time.RFC3339Nano))
	}
	if !f.To.IsZero() {
		parts = append(parts, `occurred_at <= ?`)
		args = append(args, f.To.UTC().Format(time.RFC3339Nano))
	}
	if t := strings.TrimSpace(f.Tool); t != "" {
		parts = append(parts, `tool_name = ?`)
		args = append(args, t)
	}
	if c := strings.TrimSpace(f.Coverage); c != "" {
		if c == CoverageUnclassified {
			parts = append(parts, `(`+groundedSQL+` AND IFNULL(coverage,'') NOT IN ('high','medium','low','not_applicable'))`)
		} else {
			parts = append(parts, `coverage = ?`)
			args = append(args, c)
		}
	}
	if st := strings.TrimSpace(f.Status); st != "" {
		parts = append(parts, `result_status = ?`)
		args = append(args, st)
	}
	if rc := strings.TrimSpace(f.RequestClass); rc != "" {
		parts = append(parts, `request_class = ?`)
		args = append(args, rc)
	}
	if r := strings.TrimSpace(f.Root); r != "" {
		parts = append(parts, `request_id IN (SELECT request_id FROM request_roots WHERE root_name = ? OR root_key = ?)`)
		args = append(args, r, r)
	}
	if len(parts) == 0 {
		return "1=1", args
	}
	return strings.Join(parts, " AND "), args
}

const groundedSQL = `result_status IN ('grounded_curated','grounded_local','grounded_mixed')`

// QuerySummary computes Overview cards.
func (s *Store) QuerySummary(ctx context.Context, f Filter) (Summary, error) {
	var out Summary
	if s == nil || s.db == nil {
		return out, fmt.Errorf("analytics unavailable")
	}
	out.TokenEstimatorVersion = TokenEstimatorVersion
	where, args := s.whereFilter(f)
	q := `SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN ` + groundedSQL + ` THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN curated_count > 0 OR result_status = 'grounded_curated' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN coverage = 'high' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN coverage = 'medium' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN coverage = 'low' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN ` + groundedSQL + ` AND IFNULL(coverage,'') NOT IN ('high','medium','low','not_applicable') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN coverage = 'not_applicable' OR coverage_applicable = 0 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN root_selection_required = 1 OR result_status = 'root_selection_required' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status = 'no_local_match' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status = 'local_insufficient' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status = 'request_error' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN ` + groundedSQL + ` AND ` + returnedTokensSQL + ` > 0 THEN ` + returnedTokensSQL + ` ELSE 0 END),0),
		AVG(CASE WHEN ` + groundedSQL + ` AND ` + returnedTokensSQL + ` > 0 THEN ` + returnedTokensSQL + ` END)
	FROM request_events WHERE ` + where
	var avg sql.NullFloat64
	err := s.db.QueryRowContext(ctx, q, args...).Scan(
		&out.TotalRequests, &out.GroundedRequests, &out.CuratedRequests,
		&out.HighCoverage, &out.MediumCoverage, &out.LowCoverage, &out.UnclassifiedCoverage,
		&out.NotApplicableCoverage,
		&out.RootSelectionRequired, &out.NoLocalMatch, &out.LocalInsufficient, &out.Errors,
		&out.LocalContextTokensServed, &avg,
	)
	if err != nil {
		return out, err
	}
	if out.TotalRequests > 0 {
		out.LocalEvidenceRate = float64(out.GroundedRequests) / float64(out.TotalRequests)
		out.RootSelectionRate = float64(out.RootSelectionRequired) / float64(out.TotalRequests)
	}
	if out.GroundedRequests > 0 {
		out.CuratedUsageRate = float64(out.CuratedRequests) / float64(out.GroundedRequests)
		out.HighCoverageRate = float64(out.HighCoverage) / float64(out.GroundedRequests)
		out.MediumCoverageRate = float64(out.MediumCoverage) / float64(out.GroundedRequests)
		out.LowCoverageRate = float64(out.LowCoverage) / float64(out.GroundedRequests)
		out.UnclassifiedCoverageRate = float64(out.UnclassifiedCoverage) / float64(out.GroundedRequests)
		out.UnclassifiedCoverageWarning = out.UnclassifiedCoverageRate > 0.20
	}
	if avg.Valid {
		v := avg.Float64
		out.AvgPackageTokens = &v
	}
	out.ReconcileSum = out.GroundedRequests + out.RootSelectionRequired + out.NoLocalMatch +
		out.LocalInsufficient + out.Errors
	out.ReconcileOK = out.ReconcileSum == out.TotalRequests
	return out, nil
}

// QueryTimeseries returns request counts by time bucket.
func (s *Store) QueryTimeseries(ctx context.Context, f Filter) ([]TimePoint, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("analytics unavailable")
	}
	fmtExpr, _ := bucketFormat(f.Bucket)
	where, args := s.whereFilter(f)
	q := fmt.Sprintf(`
		SELECT strftime('%s', occurred_at) AS b,
			COUNT(*),
			COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN result_status = 'root_selection_required' OR root_selection_required = 1 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN result_status = 'no_local_match' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN result_status = 'local_insufficient' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN result_status = 'request_error' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN coverage = 'high' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN coverage = 'medium' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN coverage = 'low' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN %s AND %s > 0 THEN %s ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN IFNULL(estimated_source_tokens,0) > 0 THEN estimated_source_tokens ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN IFNULL(estimated_tokens_avoided,0) > 0 THEN estimated_tokens_avoided ELSE 0 END),0),
			AVG(CASE WHEN %s AND %s > 0 THEN %s END),
			AVG(CASE WHEN IFNULL(estimated_source_tokens,0) > 0 THEN context_reduction_percent END)
		FROM request_events WHERE %s
		GROUP BY b ORDER BY b`,
		fmtExpr, groundedSQL, groundedSQL, returnedTokensSQL, returnedTokensSQL,
		groundedSQL, returnedTokensSQL, returnedTokensSQL, where)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimePoint
	for rows.Next() {
		var p TimePoint
		var avgPkg, avgRed sql.NullFloat64
		if err := rows.Scan(&p.Bucket, &p.Total, &p.Grounded, &p.RootChoice, &p.NoMatch, &p.Insufficient, &p.Errors,
			&p.High, &p.Medium, &p.Low, &p.TokensServed, &p.SourceTokens, &p.TokensAvoided, &avgPkg, &avgRed); err != nil {
			return nil, err
		}
		if avgPkg.Valid {
			v := avgPkg.Float64
			p.AvgPackage = &v
		}
		if avgRed.Valid {
			v := avgRed.Float64
			p.AvgReduction = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func bucketFormat(bucket string) (fmtExpr string, name string) {
	switch strings.ToLower(strings.TrimSpace(bucket)) {
	case "hour":
		return `%Y-%m-%dT%H:00:00Z`, "hour"
	case "week":
		return `%Y-W%W`, "week"
	case "month":
		return `%Y-%m`, "month"
	default:
		return `%Y-%m-%d`, "day"
	}
}

// QueryCoverageBreakdown returns explicit coverage / status bars.
func (s *Store) QueryCoverageBreakdown(ctx context.Context, f Filter) (CoverageBreakdown, error) {
	sum, err := s.QuerySummary(ctx, f)
	if err != nil {
		return CoverageBreakdown{}, err
	}
	return CoverageBreakdown{
		High:                  sum.HighCoverage,
		Medium:                sum.MediumCoverage,
		Low:                   sum.LowCoverage,
		Unclassified:          sum.UnclassifiedCoverage,
		NotApplicable:         sum.NotApplicableCoverage,
		LocalInsufficient:     sum.LocalInsufficient,
		NoLocalMatch:          sum.NoLocalMatch,
		RootSelectionRequired: sum.RootSelectionRequired,
		Errors:                sum.Errors,
		Grounded:              sum.GroundedRequests,
		Total:                 sum.TotalRequests,
		UnclassifiedWarning:   sum.UnclassifiedCoverageWarning,
	}, nil
}

// QueryOutcomes returns mutually exclusive request outcome classification.
func (s *Store) QueryOutcomes(ctx context.Context, f Filter) (OutcomeBreakdown, error) {
	g, err := s.QueryGrounding(ctx, f)
	if err != nil {
		return OutcomeBreakdown{}, err
	}
	return g.Outcomes, nil
}

// QueryEvidence returns overlapping evidence usage counts.
func (s *Store) QueryEvidence(ctx context.Context, f Filter) (EvidenceUsage, error) {
	g, err := s.QueryGrounding(ctx, f)
	if err != nil {
		return EvidenceUsage{}, err
	}
	return g.Evidence, nil
}

// QueryGrounding returns exclusive outcomes plus overlapping evidence usage.
func (s *Store) QueryGrounding(ctx context.Context, f Filter) (GroundingReport, error) {
	var out GroundingReport
	if s == nil || s.db == nil {
		return out, fmt.Errorf("analytics unavailable")
	}
	where, args := s.whereFilter(f)
	q := `SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN result_status = 'request_error' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status != 'request_error'
			AND (result_status = 'root_selection_required' OR root_selection_required = 1) THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status = 'no_local_match' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status = 'local_insufficient' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status = 'grounded_mixed' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status = 'grounded_curated' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN result_status = 'grounded_local' THEN 1 ELSE 0 END),0)
	FROM request_events WHERE ` + where
	err := s.db.QueryRowContext(ctx, q, args...).Scan(
		&out.Outcomes.Total, &out.Outcomes.Errors, &out.Outcomes.RootChoice,
		&out.Outcomes.NoMatch, &out.Outcomes.Insufficient, &out.Outcomes.Mixed,
		&out.Outcomes.Curated, &out.Outcomes.Local,
	)
	if err != nil {
		return out, err
	}
	classified := out.Outcomes.Errors + out.Outcomes.RootChoice + out.Outcomes.NoMatch +
		out.Outcomes.Insufficient + out.Outcomes.Mixed + out.Outcomes.Curated + out.Outcomes.Local
	if out.Outcomes.Total > classified {
		out.Outcomes.Other = out.Outcomes.Total - classified
	}

	eq := `SELECT
		COALESCE(SUM(CASE WHEN curated_count > 0 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN recipe_count > 0 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN symbol_count > 0 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN citation_count > 0 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN request_id IN (
			SELECT request_id FROM evidence_events WHERE evidence_type = 'document'
		) THEN 1 ELSE 0 END),0)
	FROM request_events WHERE ` + where
	err = s.db.QueryRowContext(ctx, eq, args...).Scan(
		&out.Evidence.Curated, &out.Evidence.Recipe, &out.Evidence.Symbol,
		&out.Evidence.RawDocuments, &out.Evidence.Document,
	)
	return out, err
}

// QueryEfficiency builds Efficiency tab metrics.
func (s *Store) QueryEfficiency(ctx context.Context, f Filter) (EfficiencyReport, error) {
	var out EfficiencyReport
	if s == nil || s.db == nil {
		return out, fmt.Errorf("analytics unavailable")
	}
	out.TokenEstimatorVersion = TokenEstimatorVersion
	where, args := s.whereFilter(f)
	var localTok, structTok, rawTok, sourceTok, avoidedTok sql.NullInt64
	var avgPkg, avgRed sql.NullFloat64
	var grounded sql.NullInt64
	q := `SELECT
		SUM(CASE WHEN ` + groundedSQL + ` AND ` + returnedTokensSQL + ` > 0 THEN ` + returnedTokensSQL + ` END),
		SUM(CASE WHEN ` + groundedSQL + ` THEN structured_tokens END),
		SUM(CASE WHEN ` + groundedSQL + ` THEN raw_document_tokens END),
		SUM(CASE WHEN ` + groundedSQL + ` AND IFNULL(estimated_source_tokens,0) > 0 THEN estimated_source_tokens END),
		SUM(CASE WHEN ` + groundedSQL + ` AND IFNULL(estimated_tokens_avoided,0) > 0 THEN estimated_tokens_avoided END),
		AVG(CASE WHEN ` + groundedSQL + ` AND ` + returnedTokensSQL + ` > 0 THEN ` + returnedTokensSQL + ` END),
		AVG(CASE WHEN ` + groundedSQL + ` AND IFNULL(estimated_source_tokens,0) > 0 THEN context_reduction_percent END),
		COALESCE(SUM(CASE WHEN ` + groundedSQL + ` THEN 1 ELSE 0 END),0)
	FROM request_events WHERE ` + where
	err := s.db.QueryRowContext(ctx, q, args...).Scan(
		&localTok, &structTok, &rawTok, &sourceTok, &avoidedTok, &avgPkg, &avgRed, &grounded,
	)
	if err != nil {
		return out, err
	}
	setI64 := func(dst **int64, n sql.NullInt64) {
		if n.Valid {
			v := n.Int64
			*dst = &v
		}
	}
	setF64 := func(dst **float64, n sql.NullFloat64) {
		if n.Valid {
			v := n.Float64
			*dst = &v
		}
	}
	setI64(&out.LocalContextTokensServed, localTok)
	setI64(&out.StructuredTokensServed, structTok)
	setI64(&out.RawDocumentTokensServed, rawTok)
	setI64(&out.EstimatedSourceTokens, sourceTok)
	setI64(&out.EstimatedTokensAvoided, avoidedTok)
	setF64(&out.AvgPackageTokens, avgPkg)
	setF64(&out.AvgContextReductionPercent, avgRed)
	if grounded.Valid {
		out.GroundedRequests = grounded.Int64
	}
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM outcome_events
		WHERE outcome IN ('implemented','success','passed')`).Scan(&out.SuccessfulOutcomes)
	if out.LocalContextTokensServed != nil && *out.LocalContextTokensServed > 0 {
		st := int64(0)
		if out.StructuredTokensServed != nil {
			st = *out.StructuredTokensServed
		}
		rt := int64(0)
		if out.RawDocumentTokensServed != nil {
			rt = *out.RawDocumentTokensServed
		}
		den := st + rt
		if den > 0 {
			share := float64(rt) / float64(den)
			out.RawDocumentShare = &share
		}
		if out.GroundedRequests > 0 {
			v := float64(*out.LocalContextTokensServed) / float64(out.GroundedRequests)
			out.TokensPerGroundedRequest = &v
		}
		if out.SuccessfulOutcomes > 0 {
			v := float64(*out.LocalContextTokensServed) / float64(out.SuccessfulOutcomes)
			out.TokensPerSuccessfulOutcome = &v
		}
	}

	// Evidence type token breakdown.
	bq := `SELECT e.evidence_type, COALESCE(SUM(e.estimated_tokens),0)
		FROM evidence_events e
		INNER JOIN request_events r ON r.request_id = e.request_id
		WHERE ` + where + `
		GROUP BY e.evidence_type ORDER BY 2 DESC`
	brows, err := s.db.QueryContext(ctx, bq, args...)
	if err != nil {
		return out, err
	}
	labels := map[string]string{
		EvidenceCurated:  "Curated knowledge",
		EvidenceRecipe:   "Recipes",
		EvidenceSymbol:   "Symbols/APIs",
		EvidenceCitation: "Raw documents",
		EvidenceDocument: "Raw documents",
	}
	var typed int64
	for brows.Next() {
		var typ string
		var tok int64
		if err := brows.Scan(&typ, &tok); err != nil {
			brows.Close()
			return out, err
		}
		label := labels[typ]
		if label == "" {
			label = typ
		}
		out.SourceTypeBreakdown = append(out.SourceTypeBreakdown, TokenTypeBucket{Type: typ, Tokens: tok, Label: label})
		typed += tok
	}
	brows.Close()
	if out.LocalContextTokensServed != nil && *out.LocalContextTokensServed > typed {
		out.SourceTypeBreakdown = append(out.SourceTypeBreakdown, TokenTypeBucket{
			Type: "overhead", Tokens: *out.LocalContextTokensServed - typed, Label: "Mixed package overhead",
		})
	}

	pts, err := s.QueryTimeseries(ctx, f)
	if err != nil {
		return out, err
	}
	out.TokenTimeseries = pts
	out.PackageTimeseries = pts
	return out, nil
}

// QueryKnowledge ranks roots and evidence keys by usage.
func (s *Store) QueryKnowledge(ctx context.Context, f Filter) (KnowledgeReport, error) {
	var out KnowledgeReport
	if s == nil || s.db == nil {
		return out, fmt.Errorf("analytics unavailable")
	}
	where, args := s.whereFilter(f)
	rq := `SELECT COALESCE(NULLIF(rr.root_key,''), rr.root_name) AS k,
			COALESCE(NULLIF(rr.root_name,''), rr.root_key) AS lab,
			COUNT(*),
			COALESCE(SUM(CASE WHEN rr.selected = 1 THEN 1 ELSE 0 END),0)
		FROM request_roots rr
		INNER JOIN request_events r ON r.request_id = rr.request_id
		WHERE ` + where + `
		GROUP BY k ORDER BY COUNT(*) DESC LIMIT 25`
	rows, err := s.db.QueryContext(ctx, rq, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var kr KnowledgeRank
		if err := rows.Scan(&kr.Key, &kr.Label, &kr.TimesSelected, &kr.TimesIncluded); err != nil {
			rows.Close()
			return out, err
		}
		out.Roots = append(out.Roots, kr)
	}
	rows.Close()

	eq := `SELECT COALESCE(NULLIF(e.evidence_key,''), e.evidence_type) AS k,
			e.evidence_type,
			COUNT(*),
			COALESCE(SUM(CASE WHEN e.included_after_trimming = 1 THEN 1 ELSE 0 END),0)
		FROM evidence_events e
		INNER JOIN request_events r ON r.request_id = e.request_id
		WHERE ` + where + `
		GROUP BY k ORDER BY COUNT(*) DESC LIMIT 25`
	erows, err := s.db.QueryContext(ctx, eq, args...)
	if err != nil {
		return out, err
	}
	for erows.Next() {
		var kr KnowledgeRank
		var typ string
		if err := erows.Scan(&kr.Key, &typ, &kr.TimesSelected, &kr.TimesIncluded); err != nil {
			erows.Close()
			return out, err
		}
		kr.Label = typ
		out.Evidence = append(out.Evidence, kr)
		if typ == EvidenceCurated || typ == EvidenceRecipe {
			out.Curated = append(out.Curated, kr)
		}
	}
	erows.Close()
	return out, nil
}

var requestSortColumns = map[string]string{
	"time":     "occurred_at",
	"tool":     "tool_name",
	"status":   "result_status",
	"coverage": "coverage",
	"tokens":   returnedTokensSQL,
	"latency":  "latency_ms",
	"sources":  "source_count",
}

// QueryRecentRequests returns newest matching requests (paginated).
func (s *Store) QueryRecentRequests(ctx context.Context, f Filter) (RequestList, error) {
	var out RequestList
	if s == nil || s.db == nil {
		return out, fmt.Errorf("analytics unavailable")
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	out.Limit = limit
	out.Offset = offset
	where, args := s.whereFilter(f)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM request_events WHERE `+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	sortCol := requestSortColumns[strings.ToLower(strings.TrimSpace(f.Sort))]
	if sortCol == "" {
		sortCol = "occurred_at"
	}
	order := "DESC"
	if strings.EqualFold(f.Order, "asc") {
		order = "ASC"
	}
	args = append(args, limit, offset)
	q := `SELECT request_id, occurred_at, tool_name, result_status, IFNULL(coverage,''),
		` + returnedTokensSQL + `, IFNULL(source_count,0), latency_ms, IFNULL(request_class,''),
		CASE WHEN curated_count > 0 THEN 1 ELSE 0 END
		FROM request_events WHERE ` + where + ` ORDER BY ` + sortCol + ` ` + order + ` LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var r RequestRow
		var curated int
		if err := rows.Scan(&r.RequestID, &r.OccurredAt, &r.ToolName, &r.ResultStatus, &r.Coverage,
			&r.ReturnedTokens, &r.SourceCount, &r.LatencyMS, &r.RequestClass, &curated); err != nil {
			rows.Close()
			return out, err
		}
		r.EstimatedTokens = r.ReturnedTokens
		r.Curated = curated == 1
		out.Requests = append(out.Requests, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	for i := range out.Requests {
		out.Requests[i].Roots = s.rootsForRequest(ctx, out.Requests[i].RequestID)
	}
	out.Count = len(out.Requests)
	return out, nil
}

// QueryRequestDetail loads one request with evidence.
func (s *Store) QueryRequestDetail(ctx context.Context, requestID string) (*RequestDetail, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("analytics unavailable")
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, fmt.Errorf("request id required")
	}
	var d RequestDetail
	var rootSel, addRet int
	var cov, fresh, fp, taskHash, errCat, errMsg sql.NullString
	var client, model, sess, reqClass, estVer sql.NullString
	var covApp sql.NullInt64
	var redPct sql.NullFloat64
	var structured, rawDoc, sourceTok, avoided int
	err := s.db.QueryRowContext(ctx, `
		SELECT request_id, occurred_at, tool_name, result_status, coverage, freshness,
			`+returnedTokensSQL+`, latency_ms, context_fingerprint, task_hash,
			root_selection_required, additional_retrieval_recommended,
			citation_count, curated_count, recipe_count, symbol_count, IFNULL(source_count,0),
			IFNULL(structured_tokens,0), IFNULL(raw_document_tokens,0),
			IFNULL(estimated_source_tokens,0), IFNULL(estimated_tokens_avoided,0),
			context_reduction_percent, token_estimator_version, coverage_applicable, request_class,
			client_name, model_name, session_hash,
			error_category, error_message
		FROM request_events WHERE request_id = ?`, requestID).Scan(
		&d.RequestID, &d.OccurredAt, &d.ToolName, &d.ResultStatus, &cov, &fresh,
		&d.ReturnedTokens, &d.LatencyMS, &fp, &taskHash,
		&rootSel, &addRet,
		&d.CitationCount, &d.CuratedCount, &d.RecipeCount, &d.SymbolCount, &d.SourceCount,
		&structured, &rawDoc, &sourceTok, &avoided,
		&redPct, &estVer, &covApp, &reqClass,
		&client, &model, &sess,
		&errCat, &errMsg,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("request not found")
	}
	if err != nil {
		return nil, err
	}
	d.EstimatedTokens = d.ReturnedTokens
	d.StructuredTokens = structured
	d.RawDocumentTokens = rawDoc
	d.EstimatedSourceTokens = sourceTok
	d.EstimatedTokensAvoided = avoided
	if redPct.Valid {
		v := redPct.Float64
		d.ContextReductionPercent = &v
	}
	d.TokenEstimatorVersion = estVer.String
	d.RequestClass = reqClass.String
	d.ClientName = client.String
	d.ModelName = model.String
	d.SessionHash = sess.String
	d.Coverage = cov.String
	d.Freshness = fresh.String
	d.ContextFingerprint = fp.String
	d.TaskHash = taskHash.String
	d.ErrorCategory = errCat.String
	d.ErrorMessage = errMsg.String
	d.RootSelectionRequired = rootSel == 1
	d.AdditionalRetrievalRecommended = addRet == 1
	d.Curated = d.CuratedCount > 0
	if covApp.Valid {
		b := covApp.Int64 == 1
		d.CoverageApplicable = &b
	}
	d.Roots = s.rootsForRequest(ctx, requestID)

	erows, err := s.db.QueryContext(ctx, `
		SELECT evidence_type, IFNULL(evidence_key,''), IFNULL(root_key,''), IFNULL(source_uri,''),
			IFNULL(authority,''), rank_position, selected_for_package, included_after_trimming,
			estimated_tokens, IFNULL(source_hash,'')
		FROM evidence_events WHERE request_id = ? ORDER BY rank_position, id`, requestID)
	if err != nil {
		return nil, err
	}
	defer erows.Close()
	for erows.Next() {
		var e EvidenceEvent
		var sel, trim int
		if err := erows.Scan(&e.EvidenceType, &e.EvidenceKey, &e.RootKey, &e.SourceURI, &e.Authority,
			&e.RankPosition, &sel, &trim, &e.EstimatedTokens, &e.SourceHash); err != nil {
			return nil, err
		}
		e.SelectedForPackage = sel == 1
		e.IncludedAfterTrimming = trim == 1
		d.Evidence = append(d.Evidence, e)
	}
	return &d, erows.Err()
}

func (s *Store) rootsForRequest(ctx context.Context, requestID string) []string {
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(root_name,''), root_key) FROM request_roots
		WHERE request_id = ? ORDER BY id`, requestID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var roots []string
	seen := map[string]bool{}
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil || !name.Valid || name.String == "" {
			continue
		}
		if seen[name.String] {
			continue
		}
		seen[name.String] = true
		roots = append(roots, name.String)
	}
	return roots
}
