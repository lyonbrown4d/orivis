package collector

import (
	"context"
	"errors"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/storx/badgerx"
	"github.com/dgraph-io/badger/v4"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

func (b *badgerResultBuffer) Push(req protocol.AgentResultRequest) ResultQueuePush {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.max == 0 {
		return ResultQueuePush{}
	}

	ctx := context.Background()
	droppedOldest := false
	deleteCount := max(0, b.size-b.max+1)
	if deleteCount > 0 {
		entries, err := b.namespace.List(ctx, badgerx.WithLimit[uint64](deleteCount))
		if err != nil {
			return ResultQueuePush{err: oops.Wrapf(err, "read badger result buffer trim keys")}
		}
		keys := collectionlist.MapList(
			collectionlist.NewList(entries...),
			func(_ int, entry badgerx.Entry[uint64, protocol.AgentResultRequest]) uint64 {
				return entry.Key
			},
		).Values()
		if len(keys) > 0 {
			if err := b.namespace.DeleteMany(ctx, keys...); err != nil {
				return ResultQueuePush{err: oops.Wrapf(err, "trim badger result buffer")}
			}
			b.size = max(0, b.size-len(keys))
			droppedOldest = true
		}
	}

	if err := b.namespace.Set(ctx, b.next, req); err != nil {
		return ResultQueuePush{err: oops.Wrapf(err, "write badger result buffer")}
	}
	b.next++
	b.size++
	return ResultQueuePush{
		size:          b.size,
		buffered:      true,
		droppedOldest: droppedOldest,
	}
}

func (b *badgerResultBuffer) Peek() (protocol.AgentResultRequest, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry, ok, err := b.namespace.First(context.Background())
	if err != nil || !ok {
		return protocol.AgentResultRequest{}, false
	}
	return entry.Value, true
}

func (b *badgerResultBuffer) PeekBatch(limit int) ([]protocol.AgentResultRequest, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if limit <= 0 {
		return nil, nil
	}

	entries, err := b.namespace.List(context.Background(), badgerx.WithLimit[uint64](limit))
	if err != nil {
		return nil, oops.Wrapf(err, "read badger result buffer batch")
	}
	return collectionlist.MapList(
		collectionlist.NewList(entries...),
		func(_ int, entry badgerx.Entry[uint64, protocol.AgentResultRequest]) protocol.AgentResultRequest {
			return entry.Value
		},
	).Values(), nil
}

func (b *badgerResultBuffer) Drop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ctx := context.Background()
	entry, ok, err := b.namespace.First(ctx)
	if err != nil {
		return oops.Wrapf(err, "read badger result buffer head")
	}
	if !ok {
		return nil
	}
	if err := b.namespace.Delete(ctx, entry.Key); err != nil {
		return oops.Wrapf(err, "drop badger result buffer head")
	}
	if b.size > 0 {
		b.size--
	}
	return nil
}

func (b *badgerResultBuffer) DropBatch(count int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if count <= 0 {
		return nil
	}

	ctx := context.Background()
	entries, err := b.namespace.List(ctx, badgerx.WithLimit[uint64](count))
	if err != nil {
		return oops.Wrapf(err, "read badger result buffer batch keys")
	}
	if len(entries) == 0 {
		return nil
	}
	keys := collectionlist.MapList(
		collectionlist.NewList(entries...),
		func(_ int, entry badgerx.Entry[uint64, protocol.AgentResultRequest]) uint64 {
			return entry.Key
		},
	).Values()
	if err := b.namespace.DeleteMany(ctx, keys...); err != nil {
		return oops.Wrapf(err, "drop badger result buffer batch")
	}
	b.size = max(0, b.size-len(keys))
	return nil
}

func (b *badgerResultBuffer) Compact(ctx context.Context) (bool, error) {
	if b == nil {
		return false, nil
	}
	b.compactMu.Lock()
	defer b.compactMu.Unlock()

	now := time.Now()
	if b.compactAt.IsZero() {
		b.compactAt = now
	}
	if now.Before(b.compactAt) {
		return false, nil
	}
	if b.compactInterval <= 0 {
		return false, nil
	}
	b.compactAt = now.Add(b.compactInterval)

	if err := ctx.Err(); err != nil {
		return false, oops.Wrapf(err, "badger result buffer compact context canceled")
	}
	if b.memory || b.db == nil {
		return false, nil
	}
	if err := b.compactValueLog(ctx); err != nil {
		return true, err
	}
	return true, nil
}

func (b *badgerResultBuffer) compactValueLog(ctx context.Context) error {
	if err := b.db.RunValueLogGC(ctx, b.compactDiscardRate); err != nil {
		if errors.Is(err, badger.ErrNoRewrite) || errors.Is(err, badger.ErrRejected) {
			return nil
		}
		return oops.Wrapf(err, "compact badger result buffer")
	}
	return nil
}

func (b *badgerResultBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.size
}

func (b *badgerResultBuffer) Close() error {
	if b == nil || b.db == nil {
		return nil
	}
	if err := b.db.Close(); err != nil {
		return oops.Wrapf(err, "close badger result buffer")
	}
	return nil
}
