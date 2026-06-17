package discovery

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lyonbrown4d/orivis/internal/protocol"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	KubernetesModeService = "service"
	KubernetesModePod     = "pod"
	KubernetesModeAll     = "all"
)

type KubernetesOptions struct {
	Mode               string
	Namespace          string
	Namespaces         []string
	Kubeconfig         string
	DefaultEnvironment string
	Logger             *slog.Logger
}

type KubernetesDiscoverer struct {
	client             kubernetes.Interface
	mode               string
	namespaces         []string
	defaultEnvironment string
	logger             *slog.Logger
}

func NewKubernetesDiscoverer(opts KubernetesOptions) (*KubernetesDiscoverer, error) {
	mode := NormalizeKubernetesMode(opts.Mode)
	if mode == "" {
		return nil, newErrorf("unsupported Kubernetes discovery mode %q", opts.Mode)
	}
	client, err := newKubernetesClient(opts.Kubeconfig)
	if err != nil {
		return nil, err
	}
	return &KubernetesDiscoverer{
		client:             client,
		mode:               mode,
		namespaces:         normalizeKubernetesNamespaces(opts.Namespace, opts.Namespaces),
		defaultEnvironment: strings.TrimSpace(opts.DefaultEnvironment),
		logger:             opts.Logger,
	}, nil
}

func (d *KubernetesDiscoverer) Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	if d == nil || d.client == nil {
		return nil, nil
	}
	if d.logger != nil {
		d.logger.Info("kubernetes discovery mode", "mode", d.mode, "namespaces", d.namespaces)
	}
	switch d.mode {
	case KubernetesModeService:
		return d.discoverServices(ctx)
	case KubernetesModePod:
		return d.discoverPods(ctx)
	case KubernetesModeAll:
		return d.discoverAll(ctx)
	default:
		return nil, newErrorf("unsupported Kubernetes discovery mode %q", d.mode)
	}
}

func (d *KubernetesDiscoverer) Close(context.Context) error {
	return nil
}

func (d *KubernetesDiscoverer) discoverAll(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	services, err := d.discoverServices(ctx)
	if err != nil {
		return nil, err
	}
	pods, err := d.discoverPods(ctx)
	if err != nil {
		return nil, err
	}
	return append(services, pods...), nil
}

func (d *KubernetesDiscoverer) discoverServices(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	items, err := d.listServices(ctx)
	if err != nil {
		return nil, err
	}
	if d.logger != nil {
		d.logger.Info("discovering kubernetes services", "count", len(items), "namespaces", d.namespaces)
	}
	return discoverByItems(
		items,
		"kubernetes_service",
		d.logger,
		d.defaultEnvironment,
		KubernetesServiceLabelSource,
		"list Kubernetes services",
	)
}

func (d *KubernetesDiscoverer) discoverPods(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	items, err := d.listPods(ctx)
	if err != nil {
		return nil, err
	}
	if d.logger != nil {
		d.logger.Info("discovering kubernetes pods", "count", len(items), "namespaces", d.namespaces)
	}
	return discoverByItems(
		items,
		"kubernetes_pod",
		d.logger,
		d.defaultEnvironment,
		KubernetesPodLabelSource,
		"list Kubernetes pods",
	)
}

func (d *KubernetesDiscoverer) listServices(ctx context.Context) ([]corev1.Service, error) {
	var out []corev1.Service
	for _, namespace := range d.discoveryNamespaces() {
		list, err := d.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, wrapErrorf(err, "list Kubernetes services namespace %q", namespace)
		}
		out = append(out, list.Items...)
	}
	return out, nil
}

func (d *KubernetesDiscoverer) listPods(ctx context.Context) ([]corev1.Pod, error) {
	var out []corev1.Pod
	for _, namespace := range d.discoveryNamespaces() {
		list, err := d.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, wrapErrorf(err, "list Kubernetes pods namespace %q", namespace)
		}
		out = append(out, list.Items...)
	}
	return out, nil
}

func (d *KubernetesDiscoverer) discoveryNamespaces() []string {
	if len(d.namespaces) == 0 {
		return []string{metav1.NamespaceAll}
	}
	return d.namespaces
}

func newKubernetesClient(kubeconfig string) (kubernetes.Interface, error) {
	cfg, err := newKubernetesRESTConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, wrapError(err, "create Kubernetes client")
	}
	return client, nil
}

func newKubernetesRESTConfig(kubeconfig string) (*rest.Config, error) {
	kubeconfig = strings.TrimSpace(kubeconfig)
	if kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, wrapError(err, "create Kubernetes config from kubeconfig")
		}
		return cfg, nil
	}
	cfg, inClusterErr := rest.InClusterConfig()
	if inClusterErr == nil {
		return cfg, nil
	}
	defaultPath := defaultKubeconfigPath()
	if defaultPath != "" {
		if _, err := os.Stat(defaultPath); err == nil {
			cfg, err := clientcmd.BuildConfigFromFlags("", defaultPath)
			if err != nil {
				return nil, wrapError(err, "create Kubernetes config from default kubeconfig")
			}
			return cfg, nil
		}
	}
	return nil, wrapError(inClusterErr, "create in-cluster Kubernetes config")
}

func defaultKubeconfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

func NormalizeKubernetesMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", KubernetesModeService, "services":
		return KubernetesModeService
	case KubernetesModePod, "pods":
		return KubernetesModePod
	case KubernetesModeAll:
		return KubernetesModeAll
	default:
		return ""
	}
}

func normalizeKubernetesNamespaces(namespace string, namespaces []string) []string {
	out := make([]string, 0, len(namespaces)+1)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if slices.Contains(out, value) {
			return
		}
		out = append(out, value)
	}
	add(namespace)
	for _, value := range namespaces {
		add(value)
	}
	return out
}
