package main

import (
	"strings"
	"testing"
)

func TestImageBindingRejectsStaleSourceAndContainerReplacement(t *testing.T) {
	digest := strings.Repeat("a", 64)
	api := buildImage{"sha256:" + strings.Repeat("b", 64), digest}
	web := buildImage{"sha256:" + strings.Repeat("c", 64), digest}
	manifest := buildManifest{digest, "fixture", map[string]buildImage{"api": api, "web": web}}
	live := []runningImage{{"api", "foodflow-acceptance", "1", api.ID, digest}, {"worker", "foodflow-acceptance", "2", api.ID, digest}, {"web", "foodflow-acceptance", "3", web.ID, digest}}
	if err := validateImageBinding(digest, manifest, live, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"old source", "old container", "wrong project", "missing service", "duplicate service", "missing platform"} {
		t.Run(name, func(t *testing.T) {
			copyLive := append([]runningImage{}, live...)
			want := digest
			platform := false
			switch name {
			case "old source":
				want = strings.Repeat("d", 64)
			case "old container":
				copyLive[0].ID = "sha256:" + strings.Repeat("e", 64)
			case "wrong project":
				copyLive[0].Project = "foodflow"
			case "missing service":
				copyLive = copyLive[:2]
			case "duplicate service":
				copyLive = append(copyLive, copyLive[0])
			case "missing platform":
				platform = true
			}
			if validateImageBinding(want, manifest, copyLive, platform) == nil {
				t.Fatal("mismatched binding accepted")
			}
		})
	}
	manifest.Images["api"] = buildImage{api.ID, "unbound"}
	if validateImageBinding(digest, manifest, live, false) == nil {
		t.Fatal("unbound image accepted")
	}
}
