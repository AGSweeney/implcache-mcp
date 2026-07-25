// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"os/exec"
	"testing"
)

type evalSummary struct {
	Top3SymbolRecall     float64 `json:"top3SymbolRecall"`
	ExpectedSourceRecall float64 `json:"expectedSourceRecall"`
	Tasks                []struct {
		ID                string `json:"id"`
		ForbiddenHit      bool   `json:"forbiddenHit"`
		DuplicateExcerpts int    `json:"duplicateExcerpts"`
		Error             string `json:"error"`
	} `json:"tasks"`
}

func TestSeedDemoRegression(t *testing.T) {
	for _, semantic := range []bool{false, true} {
		args := []string{"run", ".", "-seed-demo"}
		if semantic {
			args = append(args, "-semantic")
		}
		cmd := exec.Command("go", args...)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("semantic=%v eval failed: %v", semantic, err)
		}
		var got evalSummary
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("semantic=%v decode eval: %v\n%s", semantic, err, out)
		}
		if got.Top3SymbolRecall < 0.9 || got.ExpectedSourceRecall < 0.9 {
			t.Fatalf("semantic=%v regression: top3=%.2f source=%.2f", semantic, got.Top3SymbolRecall, got.ExpectedSourceRecall)
		}
		for _, task := range got.Tasks {
			if task.Error != "" || task.ForbiddenHit || task.DuplicateExcerpts != 0 {
				t.Fatalf("semantic=%v task=%s error=%q forbidden=%v duplicates=%d",
					semantic, task.ID, task.Error, task.ForbiddenHit, task.DuplicateExcerpts)
			}
		}
	}
}
