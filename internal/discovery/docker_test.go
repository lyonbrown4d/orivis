package discovery_test

import (
	"testing"

	"github.com/lyonbrown4d/orivis/internal/discovery"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/swarm"
)

func TestContainerSourceKey(t *testing.T) {
	key := discovery.ContainerSourceKey(container.Summary{
		ID:    "1234567890abcdef",
		Names: []string{"/web-1"},
	})
	if key != "docker:container:web-1" {
		t.Fatalf("unexpected container source key: %q", key)
	}
}

func TestContainerSourceKeyPrefersComposeService(t *testing.T) {
	key := discovery.ContainerSourceKey(container.Summary{
		ID:    "1234567890abcdef",
		Names: []string{"/project-web-1"},
		Labels: map[string]string{
			"com.docker.compose.project": "project",
			"com.docker.compose.service": "web",
		},
	})
	if key != "docker:compose:project:web" {
		t.Fatalf("unexpected compose source key: %q", key)
	}
}

func TestServiceSourceKey(t *testing.T) {
	key := discovery.ServiceSourceKey(swarm.Service{
		ID: "abcdef1234567890",
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "web",
				Labels: map[string]string{
					"com.docker.stack.namespace": "stack",
				},
			},
		},
	})
	if key != "docker:swarm:stack:web" {
		t.Fatalf("unexpected service source key: %q", key)
	}
}
