package discovery

import (
	"fmt"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionset "github.com/arcgolabs/collectionx/set"
	corev1 "k8s.io/api/core/v1"
)

func KubernetesServiceLabelSource(item corev1.Service) LabelSource {
	return LabelSource{
		SourceKey:          KubernetesServiceSourceKey(item),
		Labels:             KubernetesObjectLabels(item.Labels, item.Annotations),
		DefaultName:        KubernetesServiceName(item),
		DefaultEnvironment: KubernetesNamespaceEnvironment(item.Namespace),
		DefaultGroup:       KubernetesObjectGroup(item.Labels, item.Namespace),
		TargetHost:         KubernetesServiceTargetHost(item),
		Ports:              KubernetesServicePorts(item),
	}
}

func KubernetesPodLabelSource(item corev1.Pod) LabelSource {
	return LabelSource{
		SourceKey:          KubernetesPodSourceKey(item),
		Labels:             KubernetesObjectLabels(item.Labels, item.Annotations),
		DefaultName:        KubernetesPodName(item),
		DefaultEnvironment: KubernetesNamespaceEnvironment(item.Namespace),
		DefaultGroup:       KubernetesObjectGroup(item.Labels, item.Namespace),
		TargetHost:         KubernetesPodTargetHost(item),
		ImageName:          KubernetesPodImageName(item),
		Ports:              KubernetesPodPorts(item),
	}
}

func KubernetesServiceSourceKey(item corev1.Service) string {
	return "kubernetes:service:" + KubernetesNamespacedName(item.Namespace, item.Name)
}

func KubernetesPodSourceKey(item corev1.Pod) string {
	return "kubernetes:pod:" + KubernetesNamespacedName(item.Namespace, item.Name)
}

func KubernetesNamespacedName(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" {
		namespace = "default"
	}
	if name == "" {
		name = "unknown"
	}
	return namespace + ":" + name
}

func KubernetesServiceName(item corev1.Service) string {
	return firstNonEmpty(
		item.Labels["app.kubernetes.io/name"],
		item.Labels["app"],
		item.Name,
	)
}

func KubernetesPodName(item corev1.Pod) string {
	return firstNonEmpty(
		item.Labels["app.kubernetes.io/name"],
		item.Labels["app"],
		item.Name,
	)
}

func KubernetesNamespaceEnvironment(namespace string) string {
	return strings.TrimSpace(namespace)
}

func KubernetesObjectGroup(labels map[string]string, namespace string) string {
	return firstNonEmpty(
		labels["orivis.group"],
		labels["app.kubernetes.io/part-of"],
		labels["app.kubernetes.io/name"],
		labels["app"],
		namespace,
	)
}

func KubernetesServiceTargetHost(item corev1.Service) string {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return ""
	}
	namespace := strings.TrimSpace(item.Namespace)
	if namespace == "" {
		return name
	}
	return fmt.Sprintf("%s.%s.svc", name, namespace)
}

func KubernetesPodTargetHost(item corev1.Pod) string {
	return strings.TrimSpace(item.Status.PodIP)
}

func KubernetesObjectLabels(labels, annotations map[string]string) map[string]string {
	out := make(map[string]string, len(labels)+len(annotations))
	for key, value := range labels {
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	for key, value := range annotations {
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func KubernetesServicePorts(item corev1.Service) []int {
	seen := collectionset.NewSetWithCapacity[int](len(item.Spec.Ports))
	return collectionlist.FilterMapList(collectionlist.NewList(item.Spec.Ports...), func(_ int, port corev1.ServicePort) (int, bool) {
		if port.Protocol != "" && port.Protocol != corev1.ProtocolTCP {
			return 0, false
		}
		value := int(port.Port)
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

func KubernetesPodPorts(item corev1.Pod) []int {
	ports := collectionlist.NewList[corev1.ContainerPort]()
	for index := range item.Spec.Containers {
		container := &item.Spec.Containers[index]
		ports.Add(container.Ports...)
	}
	seen := collectionset.NewSetWithCapacity[int](ports.Len())
	return collectionlist.FilterMapList(ports, func(_ int, port corev1.ContainerPort) (int, bool) {
		if port.Protocol != "" && port.Protocol != corev1.ProtocolTCP {
			return 0, false
		}
		value := int(port.ContainerPort)
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

func KubernetesPodImageName(item corev1.Pod) string {
	for index := range item.Spec.Containers {
		container := &item.Spec.Containers[index]
		image := dockerImageName(container.Image)
		if image != "" {
			return image
		}
	}
	return ""
}
