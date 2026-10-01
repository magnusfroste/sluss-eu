package buildinfo

import "testing"

func TestShortTrimsAndKeepsDirty(t *testing.T) {
	cases := map[string]string{
		"770b436a51e6f08033a30d536312307075c7c5ff":       "770b436",
		"770b436a51e6f08033a30d536312307075c7c5ff-dirty": "770b436-dirty",
		"dev": "dev",
	}
	for in, want := range cases {
		Commit = in
		if got := Short(); got != want {
			t.Errorf("Short(%q) = %q, want %q", in, got, want)
		}
	}
}
