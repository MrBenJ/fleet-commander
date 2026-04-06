package fleet

import "testing"

func TestTmuxPrefix_WithShortName(t *testing.T) {
	f := &Fleet{ShortName: "myrepo"}
	got := f.TmuxPrefix()
	want := "fleet-myrepo"
	if got != want {
		t.Errorf("TmuxPrefix() = %q, want %q", got, want)
	}
}

func TestTmuxPrefix_WithoutShortName(t *testing.T) {
	f := &Fleet{ShortName: ""}
	got := f.TmuxPrefix()
	want := "fleet"
	if got != want {
		t.Errorf("TmuxPrefix() = %q, want %q", got, want)
	}
}
