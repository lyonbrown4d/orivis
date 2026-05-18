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

func TestContainerLabelSourceUsesComposeMetadata(t *testing.T) {
	source := discovery.ContainerLabelSource(container.Summary{
		ID:    "1234567890abcdef",
		Names: []string{"/project-redis-1"},
		Labels: map[string]string{
			"com.docker.compose.project": "project",
			"com.docker.compose.service": "redis",
			"orivis.enable":              "true",
		},
		Ports: []container.PortSummary{{PrivatePort: 6379, Type: "tcp"}},
	})
	if source.SourceKey != "docker:compose:project:redis" {
		t.Fatalf("unexpected source key: %#v", source)
	}
	if source.DefaultName != "redis" || source.DefaultEnvironment != "project" || source.TargetHost != "redis" {
		t.Fatalf("unexpected source metadata: %#v", source)
	}
	if source.DefaultGroup != "project" {
		t.Fatalf("unexpected source group: %#v", source)
	}
	if len(source.Ports) != 1 || source.Ports[0] != 6379 {
		t.Fatalf("unexpected source ports: %#v", source)
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

func TestServiceLabelSourceUsesSwarmStackGroup(t *testing.T) {
	source := discovery.ServiceLabelSource(swarm.Service{
		ID: "abcdef1234567890",
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "web",
				Labels: map[string]string{
					"com.docker.stack.namespace": "stack",
					"orivis.enable":              "true",
				},
			},
		},
	})
	if source.SourceKey != "docker:swarm:stack:web" {
		t.Fatalf("unexpected source key: %#v", source)
	}
	if source.DefaultName != "web" || source.DefaultEnvironment != "stack" || source.DefaultGroup != "stack" {
		t.Fatalf("unexpected source metadata: %#v", source)
	}
}
