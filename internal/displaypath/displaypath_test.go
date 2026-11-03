package displaypath

import (
	"path/filepath"
	"testing"
)

func TestFold(t *testing.T) {
	home := filepath.Join(t.TempDir(), "dev")
	t.Setenv("HOME", home)
	tests := map[string]string{
		home:                                "~",
		filepath.Join(home, "projects/app"): filepath.Join("~", "projects/app"),
		home + "eloper/app":                 home + "eloper/app",
		"/opt/data":                         "/opt/data",
	}
	for path, want := range tests {
		if got := Fold(path); got != want {
			t.Errorf("Fold(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestFoldText(t *testing.T) {
	home := filepath.Join(t.TempDir(), "dev")
	t.Setenv("HOME", home)
	text := "opening " + filepath.Join(home, "a.jsonl") + " and " + home + "eloper/b and /opt/c and /srv/backup" + home + "/x, path=" + home + "/y"
	want := "opening " + filepath.Join("~", "a.jsonl") + " and " + home + "eloper/b and /opt/c and /srv/backup" + home + "/x, path=~/y"
	if got := FoldText(text); got != want {
		t.Errorf("FoldText = %q, want %q", got, want)
	}
}

func TestFoldTextAtStart(t *testing.T) {
	home := filepath.Join(t.TempDir(), "dev")
	t.Setenv("HOME", home)
	if got := FoldText(home + "/a"); got != "~/a" {
		t.Errorf("FoldText at the start = %q", got)
	}
}

func TestFoldWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	if got := Fold("/x/y"); got != "/x/y" {
		t.Errorf("Fold without a home directory = %q", got)
	}
	if got := FoldText("at /x/y"); got != "at /x/y" {
		t.Errorf("FoldText without a home directory = %q", got)
	}
}
