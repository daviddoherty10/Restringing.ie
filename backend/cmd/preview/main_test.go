package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPreviewCLIValidatesBeforeConnecting(t *testing.T) {
	var out, errOut bytes.Buffer
	t.Setenv("DATABASE_URL", "postgres://production.example/production")
	if err := run([]string{"validate"}, &out, &errOut); err != nil {
		t.Fatal("normal DATABASE_URL should not affect preview command", err)
	}
	if !strings.Contains(out.String(), "accepted") {
		t.Fatal("missing validation output")
	}
	for _, args := range [][]string{nil, {"unknown"}, {"seed", "-database-url", "postgres://localhost/restringing"}, {"session", "-database-url", "postgres://remote.example/restringing_preview"}, {"seed", "unexpected"}} {
		if err := run(args, &out, &errOut); err == nil {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
}
