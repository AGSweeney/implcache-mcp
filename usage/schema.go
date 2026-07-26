// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import _ "embed"

// SchemaVersion is the usage DB PRAGMA user_version.
const SchemaVersion = 2

// TokenEstimatorVersion identifies the estimator used when writing token fields.
const TokenEstimatorVersion = "chars_div_4_v1"

//go:embed schema.sql
var schemaSQL string
