// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"strings"

	"implcache-mcp/store"
)

// TrustDecision is the mapped document trust for Upsert/Update.
type TrustDecision struct {
	Authority  string
	Deprecated bool
}

// MapTrust maps DocMeta + package state to document authority/deprecated.
func MapTrust(pkgState string, dm DocMeta) TrustDecision {
	if !dm.LibraryDocs {
		return TrustDecision{Authority: ""}
	}
	status := strings.ToLower(dm.Status)
	ev := strings.ToUpper(dm.EvidenceLevel)

	if status == "deprecated" {
		return TrustDecision{Authority: store.AuthorityUnknown, Deprecated: true}
	}

	// Invalid packages never get full curated preference.
	if pkgState == StateInvalid {
		if status == "draft" || status == "experimental" || ev == "E4" {
			return TrustDecision{Authority: store.AuthorityUnknown, Deprecated: false}
		}
		return TrustDecision{Authority: store.AuthorityOfficialDocs, Deprecated: false}
	}

	if pkgState == StateValidated && status == "verified" && (ev == "E1" || ev == "E2") {
		return TrustDecision{Authority: store.AuthorityCuratedRecipe, Deprecated: false}
	}

	if status == "verified" && ev != "E1" && ev != "E2" && ev != "" {
		// verified without strong evidence — reduced relative to full curated
		return TrustDecision{Authority: store.AuthorityOfficialDocs, Deprecated: false}
	}

	if status == "inferred" || ev == "E3" {
		return TrustDecision{Authority: store.AuthorityOfficialDocs, Deprecated: false}
	}

	if status == "draft" || status == "experimental" || ev == "E4" {
		return TrustDecision{Authority: store.AuthorityUnknown, Deprecated: false}
	}

	// Default LibraryDocs documentation
	switch dm.ContentClass {
	case ClassIndex, ClassInventory, ClassValidation:
		return TrustDecision{Authority: store.AuthorityGeneratedSummary, Deprecated: false}
	default:
		return TrustDecision{Authority: store.AuthorityOfficialDocs, Deprecated: false}
	}
}
