package cmd

import (
	"reflect"
	"testing"
)

func TestNormalizeAPIName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "cloudbuild", want: "cloudbuild.googleapis.com"},
		{in: "  Run.GoogleApis.Com ", want: "run.googleapis.com"},
		{in: "", want: ""},
	}

	for _, tt := range tests {
		if got := normalizeAPIName(tt.in); got != tt.want {
			t.Fatalf("normalizeAPIName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeAPIList_DedupAndOrder(t *testing.T) {
	t.Parallel()

	in := []string{"cloudbuild", "run.googleapis.com", "cloudbuild.googleapis.com", "RUN.GOOGLEAPIS.COM", "  ", "monitoring"}
	want := []string{"cloudbuild.googleapis.com", "run.googleapis.com", "monitoring.googleapis.com"}

	got := normalizeAPIList(in)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeAPIList mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}
