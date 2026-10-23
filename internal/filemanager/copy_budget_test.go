package filemanager

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type copyGrowthContext struct {
	context.Context
	calls int
	grow  func()
}

func (c *copyGrowthContext) Err() error {
	c.calls++
	if c.calls == 2 {
		c.grow()
	}
	return c.Context.Err()
}

func TestCopyBudgetBoundsActualBytesWhenSourceGrows(t *testing.T) {
	for _, test := range []struct {
		name                 string
		growth, prior, limit int64
		wantError            bool
	}{
		{"overflow", 64, 0, 16, true},
		{"shared-budget", 8, 8, 16, true},
		{"exact-budget", 7, 8, 16, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, root := newTestManager(t)
			source := filepath.Join(root, "input")
			mustWrite(t, source, "a")
			ctx := &copyGrowthContext{Context: context.Background(), grow: func() {
				file, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if _, err := file.Write(bytes.Repeat([]byte("b"), int(test.growth))); err != nil {
					t.Fatal(err)
				}
			}}
			budget := &copyBudget{bytes: test.prior, maxEntries: 10, maxBytes: test.limit}
			err := m.copyTreeWithAccess(ctx, "/input", "/output", budget, false)
			if test.wantError {
				if !errors.Is(err, ErrTooLarge) {
					t.Fatalf("copy error = %v", err)
				}
				if _, err := os.Stat(filepath.Join(root, "output")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("partial output remains: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if budget.bytes > test.limit {
				t.Fatalf("charged %d exceeds %d", budget.bytes, test.limit)
			}
			if !test.wantError && budget.bytes != test.prior+1+test.growth {
				t.Fatalf("actual charge = %d", budget.bytes)
			}
		})
	}
}
