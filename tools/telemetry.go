// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package tools

import (
	"implcache-mcp/usage"
)

func (opt Options) recordUsage(ev usage.RequestEvent) {
	defer func() { _ = recover() }()
	if opt.Usage == nil {
		return
	}
	opt.Usage.Record(ev)
}
