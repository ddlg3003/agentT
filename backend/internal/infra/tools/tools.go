// Package tools provides the agent's read-only data tools (and the single
// deliberate write tool used by the follow-up loop). Each type implements the
// agent.Tool port. Today they read mock JSON/markdown from a base directory;
// swapping in a real BI/Jira/Gitlab client later means changing only the Run
// method — the agent loop and prompts stay identical.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

// mockFS resolves files under a base mock directory shared by the data tools.
type mockFS struct {
	base string
}

// readFile reads a file relative to the mock base directory.
func (m mockFS) readFile(parts ...string) ([]byte, error) {
	p := filepath.Join(append([]string{m.base}, parts...)...)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read mock file %s: %w", p, err)
	}
	return b, nil
}
