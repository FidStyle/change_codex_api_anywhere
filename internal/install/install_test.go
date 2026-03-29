package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectUnixTargetDirPrefersWritableSystemPath(t *testing.T) {
	t.Parallel()

	dir, notes := selectUnixTargetDir(
		[]string{"/usr/bin", "/usr/local/bin"},
		"/home/test",
		func(path string) bool { return path == "/usr/local/bin" },
	)

	if dir != "/usr/local/bin" {
		t.Fatalf("dir = %q, want /usr/local/bin", dir)
	}

	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want empty", notes)
	}
}

func TestSelectUnixTargetDirFallsBackToUserBin(t *testing.T) {
	t.Parallel()

	dir, notes := selectUnixTargetDir(
		[]string{"/usr/bin"},
		"/home/test",
		func(path string) bool { return false },
	)

	want := filepath.Join("/home/test", ".local", "bin")
	if dir != want {
		t.Fatalf("dir = %q, want %q", dir, want)
	}

	if len(notes) == 0 || !strings.Contains(notes[0], "no writable system PATH directory found") {
		t.Fatalf("notes = %#v, want fallback note", notes)
	}
}

func TestPathContainsWithSeparatorIgnoresWindowsCase(t *testing.T) {
	t.Parallel()

	contains := pathContainsWithSeparator(`C:\Windows;C:\Users\Test\AppData\Local\ccaa\bin`, `c:\users\test\appdata\local\ccaa\bin`, ';', strings.EqualFold)
	if !contains {
		t.Fatalf("pathContainsWithSeparator() = false, want true")
	}
}
