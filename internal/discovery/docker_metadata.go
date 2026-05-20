package discovery

import (
	"slices"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionset "github.com/arcgolabs/collectionx/set"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/swarm"
)

// ContainerLabelSource returns label parsing metadata for a Docker container.
func ContainerLabelSource(item container.Summary) LabelSource {
	return LabelSource{
		SourceKey:          ContainerSourceKey(item),
		Labels:             item.Labels,
		DefaultName:        ContainerName(item),
		DefaultEnvironment: ContainerEnvironment(item),
		DefaultGroup:       ContainerGroup(item),
		TargetHost:         ContainerTargetHost(item),
		ImageName:          dockerImageName(item.Image),
		Ports:              ContainerPorts(item),
	}
}

// ContainerSourceKey returns the stable discovery source key for a Docker container.
func ContainerSourceKey(item container.Summary) string {
	if project := strings.TrimSpace(item.Labels["com.docker.compose.project"]); project != "" {
		if service := strings.TrimSpace(item.Labels["com.docker.compose.service"]); service != "" {
			return "docker:compose:" + project + ":" + service
		}
	}

	if len(item.Names) > 0 {
		name := strings.Trim(strings.TrimSpace(item.Names[0]), "/")
		if name != "" {
			return "docker:container:" + name
		}
	}

	id := shortDockerID(item.ID)
	if id == "" {
		id = "unknown"
	}
	return "docker:container:" + id
}

// ContainerName returns the best user-facing monitor name for a Docker container.
func ContainerName(item container.Summary) string {
	return firstNonEmpty(
		item.Labels["com.docker.compose.service"],
		item.Labels["com.docker.swarm.service.name"],
		containerRuntimeName(item),
		shortDockerID(item.ID),
	)
}

// ContainerEnvironment returns the best environment fallback for a Docker container.
func ContainerEnvironment(item container.Summary) string {
	return firstNonEmpty(
		item.Labels["com.docker.compose.project"],
		item.Labels["com.docker.stack.namespace"],
	)
}

// ContainerGroup returns the best dashboard group fallback for a Docker container.
func ContainerGroup(item container.Summary) string {
	return firstNonEmpty(
		item.Labels["com.docker.stack.namespace"],
		item.Labels["com.docker.compose.project"],
	)
}

// ContainerTargetHost returns the DNS name an agent can use from the same Docker network.
func ContainerTargetHost(item container.Summary) string {
	return firstNonEmpty(
		item.Labels["com.docker.compose.service"],
		item.Labels["com.docker.swarm.service.name"],
		containerRuntimeName(item),
		shortDockerID(item.ID),
	)
}

// ContainerPorts returns exposed private container ports.
func ContainerPorts(item container.Summary) []int {
	seen := collectionset.NewSetWithCapacity[int](len(item.Ports))
	return collectionlist.FilterMapList(collectionlist.NewList(item.Ports...), func(_ int, port container.PortSummary) (int, bool) {
		if !strings.EqualFold(strings.TrimSpace(port.Type), "tcp") {
			return 0, false
		}
		value := int(port.PrivatePort)
		if value == 0 {
			return 0, false
		}
		if seen.Contains(value) {
			return 0, false
		}
		seen.Add(value)
		return value, true
	}).Values()
}

func enrichContainerPortsFromInspect(item container.Summary, exposed network.PortSet) container.Summary {
	if len(ContainerPorts(item)) > 0 {
		return item
	}
	if len(exposed) == 0 {
		return item
	}

	item.Ports = append(item.Ports, exposedPortsToSummaries(exposed)...)
	return item
}

func exposedPortsToSummaries(exposed network.PortSet) []container.PortSummary {
	seen := collectionset.NewSetWithCapacity[int](len(exposed))
	raw := make([]uint16, 0, len(exposed))
	for port := range exposed {
		if strings.TrimSpace(string(port.Proto())) != "tcp" {
			continue
		}
		value := port.Num()
		if value == 0 {
			continue
		}
		valueInt := int(value)
		if seen.Contains(valueInt) {
			continue
		}
		seen.Add(valueInt)
		raw = append(raw, value)
	}

	slices.Sort(raw)
	summaries := collectionlist.NewListWithCapacity[container.PortSummary](len(raw))
	for _, value := range raw {
		summaries.Add(container.PortSummary{
			PrivatePort: value,
			Type:        "tcp",
		})
	}
	return summaries.Values()
}

// ServiceSourceKey returns the stable discovery source key for a Docker Swarm service.
func ServiceSourceKey(item swarm.Service) string {
	if namespace := strings.TrimSpace(item.Spec.Labels["com.docker.stack.namespace"]); namespace != "" {
		if item.Spec.Name != "" {
			return "docker:swarm:" + namespace + ":" + item.Spec.Name
		}
	}
	if item.Spec.Name != "" {
		return "docker:swarm:" + item.Spec.Name
	}
	id := shortDockerID(item.ID)
	if id == "" {
		id = "unknown"
	}
	return "docker:swarm:" + id
}

// ServiceLabelSource returns label parsing metadata for a Docker Swarm service.
func ServiceLabelSource(item swarm.Service) LabelSource {
	imageName := ""
	if item.Spec.TaskTemplate.ContainerSpec != nil {
		imageName = dockerImageName(item.Spec.TaskTemplate.ContainerSpec.Image)
	}
	return LabelSource{
		SourceKey:          ServiceSourceKey(item),
		Labels:             item.Spec.Labels,
		DefaultName:        ServiceName(item),
		DefaultEnvironment: ServiceEnvironment(item),
		DefaultGroup:       ServiceGroup(item),
		TargetHost:         ServiceTargetHost(item),
		ImageName:          imageName,
		Ports:              ServicePorts(item),
	}
}

func dockerImageName(rawImage string) string {
	image := strings.TrimSpace(rawImage)
	if image == "" {
		return ""
	}

	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	if slash := strings.LastIndex(image, "/"); slash >= 0 {
		image = image[slash+1:]
	}
	if colon := strings.LastIndex(image, ":"); colon >= 0 {
		image = image[:colon]
	}
	return strings.TrimSpace(image)
}

// ServiceName returns the best user-facing monitor name for a Docker Swarm service.
func ServiceName(item swarm.Service) string {
	return firstNonEmpty(item.Spec.Name, shortDockerID(item.ID))
}

// ServiceEnvironment returns the best environment fallback for a Docker Swarm service.
func ServiceEnvironment(item swarm.Service) string {
	return firstNonEmpty(item.Spec.Labels["com.docker.stack.namespace"])
}

// ServiceGroup returns the best dashboard group fallback for a Docker Swarm service.
func ServiceGroup(item swarm.Service) string {
	return firstNonEmpty(item.Spec.Labels["com.docker.stack.namespace"])
}

// ServiceTargetHost returns the DNS name an agent can use inside a Docker Swarm network.
func ServiceTargetHost(item swarm.Service) string {
	return ServiceName(item)
}

// ServicePorts returns exposed target ports for a Docker Swarm service.
func ServicePorts(item swarm.Service) []int {
	seen := collectionset.NewSetWithCapacity[int](len(item.Endpoint.Ports))
	return collectionlist.FilterMapList(collectionlist.NewList(item.Endpoint.Ports...), func(_ int, port swarm.PortConfig) (int, bool) {
		if !strings.EqualFold(string(port.Protocol), "tcp") {
			return 0, false
		}
		value := int(port.TargetPort)
		if value == 0 {
			return 0, false
		}
		if seen.Contains(value) {
			return 0, false
		}
		seen.Add(value)
		return value, true
	}).Values()
}
