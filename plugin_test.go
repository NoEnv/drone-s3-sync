package main

import (
	"reflect"
	"testing"
)

func TestListPrefix(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"/":               "",
		"app/feature/x":   "app/feature/x/",
		"app/feature/x/":  "app/feature/x/",
		"/app/feature/x/": "app/feature/x/",
	}
	for in, want := range cases {
		if got := listPrefix(in); got != want {
			t.Errorf("listPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeleteCandidatesIgnoresSiblingPrefixes(t *testing.T) {
	remote := []string{
		"app/feature/x/index.html",
		"app/feature/x/old.js",
		"app/feature/x-other/index.html",
		"app/feature/x-other/main.js",
		"app/feature/xy/index.html",
	}
	local := []string{"index.html"}

	got := deleteCandidates(remote, local, "app/feature/x")
	want := []string{"app/feature/x/old.js"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deleteCandidates() = %v, want %v", got, want)
	}
}

func TestDeleteCandidatesTrailingSlashTarget(t *testing.T) {
	remote := []string{"app/x/a.js", "app/x/sub/b.js", "app/x2/a.js"}
	local := []string{"sub/b.js"}

	got := deleteCandidates(remote, local, "/app/x/")
	want := []string{"app/x/a.js"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deleteCandidates() = %v, want %v", got, want)
	}
}

func TestDeleteCandidatesBucketRoot(t *testing.T) {
	remote := []string{"index.html", "old.html", "assets/a.js"}
	local := []string{"index.html", "assets/a.js"}

	got := deleteCandidates(remote, local, "")
	want := []string{"old.html"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deleteCandidates() = %v, want %v", got, want)
	}
}
