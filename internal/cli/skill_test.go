package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSkillPrintsGuideWithoutJSON(t *testing.T) {
	var out, err bytes.Buffer
	cmd := newRoot()
	cmd.SetOut(&out)
	cmd.SetErr(&err)
	cmd.SetArgs([]string{"skill"})
	if execErr := cmd.Execute(); execErr != nil {
		t.Fatal(execErr)
	}
	if !strings.Contains(out.String(), "You do not pass `--as primary`") {
		t.Fatalf("%s", out.String())
	}
	if strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Fatal("skill output was JSON")
	}
}

func TestInstalledSkillContainsBinaryGuide(t *testing.T) {
	body, err := os.ReadFile("../../SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), strings.TrimSpace(skillText)) {
		t.Fatal("SKILL.md does not contain the binary guide")
	}
}
