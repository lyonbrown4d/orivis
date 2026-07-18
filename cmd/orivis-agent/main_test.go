package main

import (
	"github.com/arcgolabs/collectionx/list"
	"testing"

	"github.com/arcgolabs/dix"
	"github.com/spf13/cobra"
)

func TestAgentAppModulesShouldIncludeCoreModules(t *testing.T) {
	app := newTestAgentApp(t, map[string]string{})

	gotModules := make(map[string]struct{}, app.Modules().Len())
	for _, module := range app.Modules().Values() {
		gotModules[module.Name()] = struct{}{}
	}

	wantModules := []string{
		"collector",
	}

	for _, moduleName := range wantModules {
		if _, ok := gotModules[moduleName]; !ok {
			t.Fatalf("module %q missing from app module set", moduleName)
		}
	}
}

func TestAgentAppDependencyGraphShouldContainCoreModuleNodes(t *testing.T) {
	app := newTestAgentApp(t, map[string]string{})
	graph, err := app.DependencyGraph()
	if err != nil {
		t.Fatalf("DependencyGraph: unexpected error: %v", err)
	}

	moduleNodes := make(map[string]struct{})
	for _, node := range graph.Nodes.Values() {
		if node.Kind != dix.DependencyGraphNodeModule {
			continue
		}
		moduleNodes[node.Module] = struct{}{}
	}

	wantNodes := []string{
		"concurrency",
		"collector",
	}
	for _, name := range wantNodes {
		if _, ok := moduleNodes[name]; !ok {
			t.Fatalf("dependency graph missing module node %q", name)
		}
	}
}

func TestAgentAppBuildFailsWhenRequiredModuleDisabled(t *testing.T) {
	app := newTestAgentApp(t, map[string]string{}).Test(dix.TestDisableModules("config"))
	_, err := app.Build()
	if err == nil {
		t.Fatalf("app.Build should fail when required module is disabled")
	}
}

func TestAgentAppLifecycleHooksShouldExposeCoreStartupAndShutdownHooks(t *testing.T) {
	app := newTestAgentApp(t, map[string]string{})

	rt, err := app.Build()
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}

	summary := rt.LifecycleSummary()

	wantStartHooks := []string{
		"start-agent-collector",
	}
	wantStopHooks := []string{
		"stop-agent-collector",
		"close-agent-task-pool",
	}

	for _, want := range wantStartHooks {
		if !containsLifecycleHook(summary.Start, want) {
			t.Fatalf("start hook %q not registered", want)
		}
	}
	for _, want := range wantStopHooks {
		if !containsLifecycleHook(summary.Stop, want) {
			t.Fatalf("stop hook %q not registered", want)
		}
	}
}

func newTestAgentApp(t *testing.T, flagValues map[string]string) *dix.App {
	t.Helper()

	cmd := &cobra.Command{Use: "orivis-agent"}
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("agent-name", "", "")
	cmd.Flags().String("agent-region", "", "")
	cmd.Flags().String("runtime", "", "")
	cmd.Flags().Duration("poll-interval", 0, "")
	cmd.Flags().Int("poll-workers", 0, "")
	cmd.Flags().String("buffer-driver", "", "")
	cmd.Flags().String("buffer-path", "", "")

	for name, value := range flagValues {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set agent flag %q=%q: %v", name, value, err)
		}
	}

	return newAgentApp(cmd, "")
}

func containsLifecycleHook(hooks *list.List[dix.LifecycleHookSummary], name string) bool {
	for _, hook := range hooks.Values() {
		if hook.Name == name {
			return true
		}
	}
	return false
}
