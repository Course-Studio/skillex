package registry

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestAllPackages_DeterministicOrder(t *testing.T) {
	reg, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()

	for _, s := range []Skill{
		{Path: "n1.md", Content: "x", Visibility: "public", SourceType: "dependency", PackageName: "@z/pkg", PackageVersion: "1.0.0"},
		{Path: "n2.md", Content: "x", Visibility: "public", SourceType: "dependency", PackageName: "@a/pkg", PackageVersion: "2.0.0"},
		{Path: "n3.md", Content: "x", Visibility: "private", SourceType: "dependency", PackageName: "@m/pkg", PackageVersion: "3.0.0"},
		// Same name as n2, lower version: forces the Name-tie -> Version comparator branch.
		{Path: "n4.md", Content: "x", Visibility: "public", SourceType: "dependency", PackageName: "@a/pkg", PackageVersion: "1.0.0"},
	} {
		if _, err := reg.InsertSkill(s); err != nil {
			t.Fatal(err)
		}
	}

	// Sorted by Name, then Version — the two @a/pkg rows pin the version tiebreak.
	want := []string{"@a/pkg@1.0.0", "@a/pkg@2.0.0", "@m/pkg@3.0.0", "@z/pkg@1.0.0"}
	for i := 0; i < 20; i++ {
		pkgs, err := reg.AllPackages()
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(pkgs))
		for j, p := range pkgs {
			got[j] = p.Name + "@" + p.Version
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d: non-deterministic or unsorted order: got %v, want %v", i, got, want)
		}
	}
}
