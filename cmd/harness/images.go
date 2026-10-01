package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type buildImage struct {
	ID           string `json:"id"`
	SourceDigest string `json:"source_digest"`
}
type buildManifest struct {
	SourceDigest string                `json:"source_digest"`
	Revision     string                `json:"revision"`
	Images       map[string]buildImage `json:"images"`
}
type runningImage struct {
	Service      string `json:"service"`
	Project      string `json:"project"`
	ContainerID  string `json:"container_id"`
	ID           string `json:"image_id"`
	SourceDigest string `json:"source_digest"`
}

var imageIDPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validateImageBinding(digest string, manifest buildManifest, live []runningImage, platform bool) error {
	if !digestPattern.MatchString(digest) || manifest.SourceDigest != digest {
		return fmt.Errorf("acceptance build manifest does not match current source; rebuild acceptance")
	}
	required := map[string]string{"api": "api", "worker": "api", "web": "web"}
	if platform {
		required["relay"] = "api"
		required["insights"] = "api"
	}
	for _, name := range []string{"api", "web"} {
		image, ok := manifest.Images[name]
		if !ok || !imageIDPattern.MatchString(image.ID) || image.SourceDigest != digest {
			return fmt.Errorf("%s image lacks matching build identity", name)
		}
	}
	seen := map[string]bool{}
	for _, image := range live {
		kind, ok := required[image.Service]
		if !ok {
			return fmt.Errorf("unexpected image binding service: %s", image.Service)
		}
		if seen[image.Service] {
			return fmt.Errorf("duplicate running service: %s", image.Service)
		}
		seen[image.Service] = true
		if image.Project != "foodflow-acceptance" || image.ID != manifest.Images[kind].ID || image.SourceDigest != digest || image.ContainerID == "" {
			return fmt.Errorf("%s running image is not the recorded source build", image.Service)
		}
	}
	for name := range required {
		if !seen[name] {
			return fmt.Errorf("required acceptance service missing: %s", name)
		}
	}
	return nil
}

// Export only image/container identities and allowlisted labels, never env/config.
func verifyAcceptanceImages(root, digest string, platform bool) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, ".cache", "harness", "acceptance-images.json"))
	if err != nil {
		return "", fmt.Errorf("acceptance build manifest unavailable; rebuild acceptance")
	}
	var manifest buildManifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return "", fmt.Errorf("invalid acceptance build manifest")
	}
	names := []string{"api", "worker", "web"}
	if platform {
		names = append(names, "relay", "insights")
	}
	live := []runningImage{}
	for _, name := range names {
		ids, e := command("docker", "ps", "--filter", "label=com.docker.compose.project=foodflow-acceptance", "--filter", "label=com.docker.compose.service="+name, "--filter", "label=com.docker.compose.oneoff=False", "--format", "{{.ID}}")
		if e != nil {
			return "", fmt.Errorf("cannot read running image identities")
		}
		for _, id := range strings.Fields(ids) {
			limited := `{"service":{{json (index .Config.Labels "com.docker.compose.service")}},"project":{{json (index .Config.Labels "com.docker.compose.project")}},"container_id":{{json .Id}},"image_id":{{json .Image}},"source_digest":{{json (index .Config.Labels "io.foodflow.source-digest")}}}`
			value, e := command("docker", "inspect", "--format", limited, id)
			if e != nil {
				return "", fmt.Errorf("cannot inspect %s image identity", name)
			}
			var image runningImage
			if e = json.Unmarshal([]byte(value), &image); e != nil {
				return "", fmt.Errorf("invalid running image identity")
			}
			live = append(live, image)
		}
	}
	if err = validateImageBinding(digest, manifest, live, platform); err != nil {
		return "", err
	}
	evidence := struct {
		SourceDigest string         `json:"source_digest"`
		Services     []runningImage `json:"services"`
	}{digest, live}
	if err = saveJSON(filepath.Join(root, ".cache", "harness", "image-verification.json"), evidence); err != nil {
		return "", err
	}
	value, _ := json.Marshal(evidence)
	return string(value), nil
}
