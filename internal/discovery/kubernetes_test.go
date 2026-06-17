package discovery_test

import (
	"testing"

	"github.com/lyonbrown4d/orivis/internal/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubernetesServiceLabelSource(t *testing.T) {
	source := discovery.KubernetesServiceLabelSource(corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana",
			Namespace: "observability",
			Labels: map[string]string{
				"app.kubernetes.io/name":    "grafana",
				"app.kubernetes.io/part-of": "monitoring",
				"orivis.enable":             "true",
			},
			Annotations: map[string]string{
				"orivis.monitor.http.type":   "http",
				"orivis.monitor.http.target": "http://grafana.observability.svc:3000/api/health",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 3000, Protocol: corev1.ProtocolTCP}},
		},
	})
	if source.SourceKey != "kubernetes:service:observability:grafana" {
		t.Fatalf("unexpected source key: %#v", source)
	}
	if source.DefaultName != "grafana" || source.DefaultEnvironment != "observability" || source.DefaultGroup != "monitoring" {
		t.Fatalf("unexpected source metadata: %#v", source)
	}
	if source.TargetHost != "grafana.observability.svc" {
		t.Fatalf("unexpected target host: %#v", source)
	}
	if len(source.Ports) != 1 || source.Ports[0] != 3000 {
		t.Fatalf("unexpected ports: %#v", source)
	}
	if source.Labels["orivis.monitor.http.target"] == "" {
		t.Fatalf("expected annotations to be available as labels: %#v", source.Labels)
	}
}

func TestKubernetesPodLabelSourceInfersHTTPFromImage(t *testing.T) {
	monitors, err := discovery.ParseLabels(discovery.KubernetesPodLabelSource(corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-6d7f",
			Namespace: "default",
			Labels: map[string]string{
				"app":           "web",
				"orivis.enable": "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Image: "nginx:1.27-alpine",
				Ports: []corev1.ContainerPort{{ContainerPort: 80, Protocol: corev1.ProtocolTCP}},
			}},
		},
		Status: corev1.PodStatus{PodIP: "10.42.0.12"},
	}))
	if err != nil {
		t.Fatalf("parse pod labels: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("expected one monitor, got %#v", monitors)
	}
	monitor := monitors[0]
	if monitor.SourceKey != "kubernetes:pod:default:web-6d7f:web" {
		t.Fatalf("unexpected monitor source: %#v", monitor)
	}
	if monitor.Type != "http" || monitor.Target != "http://10.42.0.12:80" || monitor.GroupName != "web" {
		t.Fatalf("unexpected inferred monitor: %#v", monitor)
	}
}

func TestNormalizeKubernetesMode(t *testing.T) {
	if discovery.NormalizeKubernetesMode("") != discovery.KubernetesModeService {
		t.Fatal("expected empty Kubernetes mode to default to service")
	}
	if discovery.NormalizeKubernetesMode("pods") != discovery.KubernetesModePod {
		t.Fatal("expected pods alias to normalize to pod")
	}
	if discovery.NormalizeKubernetesMode("invalid") != "" {
		t.Fatal("expected invalid Kubernetes mode to be rejected")
	}
}
