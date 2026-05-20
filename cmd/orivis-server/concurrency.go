package main

import (
	"context"

	"github.com/arcgolabs/dix"
	"github.com/lyonbrown4d/orivis/internal/concurrency"
	"github.com/panjf2000/ants/v2"
)

func newServerConcurrencyModule(configModule, loggingModule dix.Module) dix.Module {
	return dix.NewModule("concurrency",
		dix.WithModuleImports(configModule, loggingModule),
		dix.WithModuleProviders(
			dix.ProviderErr0(concurrency.DefaultWorkerPool),
		),
		dix.WithModuleHooks(
			dix.OnStop[*ants.Pool](func(_ context.Context, pool *ants.Pool) error {
				pool.Release()
				return nil
			}, dix.LifecycleName("close-server-task-pool")),
		),
	)
}
