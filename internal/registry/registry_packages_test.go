package registry

import (
	"path/filepath"
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
	} {
		if _, err := reg.InsertSkill(s); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 20; i++ {
		pkgs, err := reg.AllPackages()
		if err != nil {
			t.Fatal(err)
		}
		if len(pkgs) != 3 || pkgs[0].Name != "@a/pkg" || pkgs[1].Name != "@m/pkg" || pkgs[2].Name != "@z/pkg" {
			t.Fatalf("iteration %d: non-deterministic or unsorted order: %v", i, pkgs)
		}
	}
}
