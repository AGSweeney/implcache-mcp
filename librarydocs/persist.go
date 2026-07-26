// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"implcache-mcp/store"
)

// PersistMeta upserts the synthetic LibraryDocs metadata document.
func PersistMeta(ctx context.Context, st *store.Store, scheme, rootName string, meta *PackageMeta) (string, error) {
	if st == nil || meta == nil {
		return "", fmt.Errorf("store and meta required")
	}
	meta.RecomputeSummary()
	body, err := meta.MarshalJSONBody()
	if err != nil {
		return "", err
	}
	uri := MetaURI(scheme, rootName)
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI:        uri,
		Title:      "LibraryDocs package metadata",
		SourceType: store.SourceMarkdown,
		Path:       MetaRelativePath,
		RootName:   rootName,
		Authority:  store.AuthorityGeneratedSummary,
		Technology: TechnologyMeta,
		Language:   "json",
		Mtime:      time.Now().Unix(),
		Hash:       hash,
		Chunks: []store.Chunk{{
			Ordinal: 0,
			Heading: "librarydocs-meta",
			Body:    string(body),
		}},
	})
	return uri, err
}

// DeleteMeta removes the synthetic metadata document if present.
func DeleteMeta(ctx context.Context, st *store.Store, scheme, rootName string) error {
	uri := MetaURI(scheme, rootName)
	_, err := st.DeleteDocument(ctx, uri)
	return err
}

// LoadMeta loads PackageMeta from the synthetic document, if any.
func LoadMeta(ctx context.Context, st *store.Store, scheme, rootName string) (*PackageMeta, error) {
	uri := MetaURI(scheme, rootName)
	doc, chunks, err := st.GetDocumentByURI(ctx, uri)
	if err != nil {
		if isDocNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if doc == nil {
		return nil, nil
	}
	var body string
	for _, c := range chunks {
		body += c.Body
	}
	return ParsePackageMeta([]byte(body))
}

func isDocNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "document not found")
}

// ApplyTrustUpdates sets authority/deprecated on LibraryDocs documents using meta.
func ApplyTrustUpdates(ctx context.Context, st *store.Store, scheme, rootName string, meta *PackageMeta) error {
	if meta == nil || meta.Handling != HandlingAuto {
		return nil
	}
	for rel, dm := range meta.Documents {
		uri := scheme + "://" + rootName + "/" + rel
		td := MapTrust(meta.PackageState, dm)
		if td.Authority == "" {
			continue
		}
		if err := st.UpdateDocumentTrust(ctx, uri, td.Authority, td.Deprecated); err != nil {
			// non-fatal during enrichment (missing docs, etc.)
			continue
		}
	}
	return nil
}
