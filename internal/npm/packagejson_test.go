package npm

import "testing"

func TestParsePackageManifestWorkspaces(t *testing.T) {
	t.Run("array form", func(t *testing.T) {
		manifest, err := ParsePackageManifest([]byte(`{"workspaces":["apps/*","packages/*"]}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(manifest.Workspaces) != 2 || manifest.Workspaces[0] != "apps/*" || manifest.Workspaces[1] != "packages/*" {
			t.Fatalf("unexpected workspaces: %#v", manifest.Workspaces)
		}
	})

	t.Run("object form", func(t *testing.T) {
		manifest, err := ParsePackageManifest([]byte(`{"workspaces":{"packages":["packages/*"]}}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(manifest.Workspaces) != 1 || manifest.Workspaces[0] != "packages/*" {
			t.Fatalf("unexpected workspaces: %#v", manifest.Workspaces)
		}
	})
}
