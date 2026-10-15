package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeTopology struct {
	Services map[string]composeTopologyService `yaml:"services"`
	Networks map[string]struct {
		Internal bool `yaml:"internal"`
	} `yaml:"networks"`
}

type composeTopologyService struct {
	Image       string                      `yaml:"image"`
	Environment map[string]string           `yaml:"environment"`
	Ports       []any                       `yaml:"ports"`
	Profiles    []string                    `yaml:"profiles"`
	Networks    any                         `yaml:"networks"`
	DependsOn   composeTopologyDependencies `yaml:"depends_on"`
	Healthcheck struct {
		Test []string `yaml:"test"`
	} `yaml:"healthcheck"`
}

type composeTopologyDependency struct {
	Condition string `yaml:"condition"`
	Required  *bool  `yaml:"required"`
}

type composeTopologyDependencies map[string]composeTopologyDependency

func (dependencies *composeTopologyDependencies) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.SequenceNode {
		var names []string
		if err := node.Decode(&names); err != nil {
			return err
		}
		*dependencies = composeTopologyDependencies{}
		for _, name := range names {
			(*dependencies)[name] = composeTopologyDependency{Condition: "service_started"}
		}
		return nil
	}
	var values map[string]composeTopologyDependency
	if err := node.Decode(&values); err != nil {
		return err
	}
	*dependencies = values
	return nil
}

func readComposeTopology(t *testing.T, name string) composeTopology {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../..", name))
	if err != nil {
		t.Fatal(err)
	}
	var topology composeTopology
	if err := yaml.Unmarshal(raw, &topology); err != nil {
		t.Fatal(err)
	}
	return topology
}

func composeServiceNetworks(t *testing.T, service composeTopologyService) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	switch networks := service.Networks.(type) {
	case nil:
		names["default"] = true
	case []any:
		for _, name := range networks {
			value, ok := name.(string)
			if !ok {
				t.Fatal("invalid Compose network name")
			}
			names[value] = true
		}
	case map[string]any:
		for name := range networks {
			names[name] = true
		}
	default:
		t.Fatal("unexpected Compose network declaration")
	}
	return names
}

// Every independently started API must have a private, required Redis path.
// In particular, learning and restored APIs cannot inherit the acceptance overlay.
func TestComposeRequiresRedisForEveryAPI(t *testing.T) {
	for _, file := range []string{"compose.yaml", "compose.acceptance.yaml", "compose.learning.yaml", "compose.restore.yaml"} {
		t.Run(file, func(t *testing.T) {
			topology := readComposeTopology(t, file)
			api := topology.Services["api"]
			redis, ok := topology.Services["redis"]
			if !ok || redis.Image == "" || len(redis.Profiles) != 0 {
				t.Fatal("Redis must start with the base API deployment")
			}
			if api.Environment["REDIS_URL"] != "redis://redis:6379/0" {
				t.Fatal("API must use its own internal Redis service")
			}
			dependency, ok := api.DependsOn["redis"]
			if !ok || dependency.Condition != "service_healthy" || (dependency.Required != nil && !*dependency.Required) {
				t.Fatal("API startup must wait for healthy required Redis")
			}
			if len(redis.Ports) != 0 {
				t.Fatal("Redis must not publish host ports")
			}
			if len(redis.Healthcheck.Test) == 0 {
				t.Fatal("Redis requires an actual health probe")
			}
			apiNetworks := composeServiceNetworks(t, api)
			redisNetworks := composeServiceNetworks(t, redis)
			shared := false
			for name := range redisNetworks {
				shared = shared || apiNetworks[name]
				if (file == "compose.learning.yaml" || file == "compose.restore.yaml") && !topology.Networks[name].Internal {
					t.Fatal("learning and restore Redis must stay on internal networks")
				}
			}
			if !shared {
				t.Fatal("API and Redis must share a service network")
			}
			// Worker queue and allowance decisions remain durable in PostgreSQL.
			if _, ok := topology.Services["worker"].DependsOn["redis"]; ok {
				t.Fatal("Worker must not acquire an artificial cache dependency")
			}
		})
	}
}

func TestComposeOverlaysPreserveRequiredRedis(t *testing.T) {
	for _, file := range []string{"compose.platform.yaml", "compose.trial.yaml", "compose.trial-deepseek.yaml"} {
		t.Run(file, func(t *testing.T) {
			topology := readComposeTopology(t, file)
			if _, ok := topology.Services["redis"]; ok {
				t.Fatal("Redis belongs to the base deployment, not an optional overlay")
			}
			api := topology.Services["api"]
			if value, ok := api.Environment["REDIS_URL"]; ok && value != "redis://redis:6379/0" {
				t.Fatal("overlay overrides the required internal Redis URL")
			}
			if dependency, ok := api.DependsOn["redis"]; ok && (dependency.Condition != "service_healthy" || (dependency.Required != nil && !*dependency.Required)) {
				t.Fatal("overlay makes Redis optional")
			}
		})
	}
}
