package main

import (
	"testing"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dix"
	"github.com/spf13/cobra"
)

func TestServerAppModulesShouldIncludeCoreModules(t *testing.T) {
	app := newTestServerApp(t, map[string]string{})
	assertAppModules(t, app, []string{"http", "mdns", "retention", "notification", "concurrency"})
}

func TestServerAppDependencyGraphShouldContainCoreModuleNodes(t *testing.T) {
	graph := mustBuildDependencyGraph(t, newTestServerApp(t, map[string]string{}))
	assertDependencyGraphHasModuleNodes(t, graph, []string{"concurrency", "cache", "store", "http-endpoints"})
}

func TestServerAppDependencyGraphShouldConnectLifecycleContextToStore(t *testing.T) {
	graph := mustBuildDependencyGraph(t, newTestServerApp(t, map[string]string{}))
	assertDependencyGraphHasImportEdge(t, graph, "lifecycle-context", "store")
}

func TestServerAppBuildFailsWhenRequiredModuleDisabled(t *testing.T) {
	app := newTestServerApp(t, map[string]string{}).Test(dix.TestDisableModules("config"))
	_, err := app.Build()
	if err == nil {
		t.Fatalf("app.Build should fail when required module is disabled")
	}
}

func TestServerAppLifecycleHooksShouldExposeCoreStartupAndShutdownHooks(t *testing.T) {
	app := newTestServerApp(t, map[string]string{})

	rt, err := app.Build()
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}

	summary := rt.LifecycleSummary()

	for _, tc := range []struct {
		name  string
		hooks *list.List[dix.LifecycleHookSummary]
		want  []string
	}{
		{
			name:  "start",
			hooks: summary.Start,
			want:  []string{"start-mdns-discovery", "start-http-server", "start-retention"},
		},
		{
			name:  "stop",
			hooks: summary.Stop,
			want:  []string{"stop-http-server", "stop-result-ingestor", "close-server-task-pool"},
		},
	} {
		assertLifecycleHooksContain(t, tc.name, tc.hooks, tc.want)
	}
}

func newTestServerApp(t *testing.T, flagValues map[string]string) *dix.App {
	t.Helper()

	cmd := &cobra.Command{Use: "orivis-server"}
	var configFile string
	registerServerFlags(cmd, &configFile)

	for name, value := range flagValues {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set server flag %q=%q: %v", name, value, err)
		}
	}

	return newServerApp(t.Context(), cmd, configFile)
}

func assertAppModules(t *testing.T, app *dix.App, moduleNames []string) {
	t.Helper()

	modules := make(map[string]struct{}, app.Modules().Len())
	for _, module := range app.Modules().Values() {
		modules[module.Name()] = struct{}{}
	}

	for _, moduleName := range moduleNames {
		if _, ok := modules[moduleName]; !ok {
			t.Fatalf("module %q missing from app module set", moduleName)
		}
	}
}

func mustBuildDependencyGraph(t *testing.T, app *dix.App) dix.DependencyGraph {
	t.Helper()

	graph, err := app.DependencyGraph()
	if err != nil {
		t.Fatalf("DependencyGraph: unexpected error: %v", err)
	}
	return graph
}

func assertDependencyGraphHasModuleNodes(t *testing.T, graph dix.DependencyGraph, wantNodes []string) {
	t.Helper()

	moduleNodes := make(map[string]struct{})
	for _, node := range graph.Nodes.Values() {
		if node.Kind != dix.DependencyGraphNodeModule {
			continue
		}
		moduleNodes[node.Module] = struct{}{}
	}

	for _, name := range wantNodes {
		if _, ok := moduleNodes[name]; !ok {
			t.Fatalf("dependency graph missing module node %q", name)
		}
	}
}

func assertDependencyGraphHasImportEdge(t *testing.T, graph dix.DependencyGraph, from, to string) {
	t.Helper()

	moduleIDs := make(map[string]string)
	for _, node := range graph.Nodes.Values() {
		if node.Kind == dix.DependencyGraphNodeModule {
			moduleIDs[node.Module] = node.ID
		}
	}

	fromID, ok := moduleIDs[from]
	if !ok {
		t.Fatalf("dependency graph missing module %q", from)
	}
	toID, ok := moduleIDs[to]
	if !ok {
		t.Fatalf("dependency graph missing module %q", to)
	}

	for _, edge := range graph.Edges.Values() {
		if edge.Kind == dix.DependencyGraphEdgeImports && edge.From == fromID && edge.To == toID {
			return
		}
	}

	t.Fatalf("expected dependency edge from %q to %q", from, to)
}

func assertLifecycleHooksContain(t *testing.T, kind string, hooks *list.List[dix.LifecycleHookSummary], want []string) {
	t.Helper()

	for _, name := range want {
		if !containsLifecycleHook(hooks, name) {
			t.Fatalf("%s hook %q not registered", kind, name)
		}
	}
}

func containsLifecycleHook(hooks *list.List[dix.LifecycleHookSummary], name string) bool {
	for _, hook := range hooks.Values() {
		if hook.Name == name {
			return true
		}
	}
	return false
}
