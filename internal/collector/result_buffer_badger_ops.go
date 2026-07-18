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

func (b *badgerResultBuffer) Push(ctx context.Context, req protocol.AgentResultRequest) ResultQueuePush {
	if b == nil {
		return ResultQueuePush{err: oops.Wrapf(ErrResultBufferClosed, "push badger result buffer")}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.operationError(ctx, "push badger result buffer"); err != nil {
		return ResultQueuePush{err: err}
	}
	if b.max == 0 {
		return ResultQueuePush{}
	}

	droppedOldest := false
	if b.size >= b.max {
		var err error
		droppedOldest, err = b.trimOldest(ctx, b.size-b.max+1)
		if err != nil {
			return ResultQueuePush{err: err}
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

func (b *badgerResultBuffer) Peek(ctx context.Context) (protocol.AgentResultRequest, bool) {
	if b == nil {
		return protocol.AgentResultRequest{}, false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.operationError(ctx, "peek badger result buffer"); err != nil {
		return protocol.AgentResultRequest{}, false
	}

	entry, ok, err := b.namespace.First(ctx)
	if err != nil || !ok {
		return protocol.AgentResultRequest{}, false
	}
	return entry.Value, true
}

func (b *badgerResultBuffer) PeekBatch(ctx context.Context, limit int) ([]protocol.AgentResultRequest, error) {
	if b == nil {
		return nil, oops.Wrapf(ErrResultBufferClosed, "read badger result buffer batch")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.operationError(ctx, "read badger result buffer batch"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []protocol.AgentResultRequest{}, nil
	}

	entries, err := b.namespace.List(ctx, badgerx.WithLimit[uint64](limit))
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

func (b *badgerResultBuffer) Drop(ctx context.Context) error {
	if b == nil {
		return oops.Wrapf(ErrResultBufferClosed, "drop badger result buffer")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.operationError(ctx, "drop badger result buffer"); err != nil {
		return err
	}

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

func (b *badgerResultBuffer) DropBatch(ctx context.Context, count int) error {
	if b == nil {
		return oops.Wrapf(ErrResultBufferClosed, "drop badger result buffer batch")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.operationError(ctx, "drop badger result buffer batch"); err != nil {
		return err
	}
	if count <= 0 {
		return nil
	}
	keys, err := b.listKeys(ctx, count, "read badger result buffer batch keys")
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
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
	if err := b.operationError(ctx, "badger result buffer compact"); err != nil {
		return false, err
	}

	b.compactMu.Lock()
	defer b.compactMu.Unlock()

	db, shouldCompact, err := b.compactCandidate()
	if err != nil || !shouldCompact {
		return false, err
	}
	if err := b.compactValueLogWithDB(ctx, db); err != nil {
		return true, err
	}
	return true, nil
}

func (b *badgerResultBuffer) compactValueLogWithDB(ctx context.Context, db *badgerx.DB) error {
	if err := db.RunValueLogGC(ctx, b.compactDiscardRate); err != nil {
		if errors.Is(err, badger.ErrNoRewrite) || errors.Is(err, badger.ErrRejected) {
			return nil
		}
		return oops.Wrapf(err, "compact badger result buffer")
	}
	return nil
}

func (b *badgerResultBuffer) Len() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return 0
	}
	return b.size
}

func (b *badgerResultBuffer) Close() error {
	if b == nil {
		return nil
	}
	b.compactMu.Lock()
	b.mu.Lock()

	if b.closed {
		b.mu.Unlock()
		b.compactMu.Unlock()
		return nil
	}
	b.closed = true
	b.size = 0
	b.next = 0
	db := b.db
	b.db = nil
	b.namespace = nil
	b.mu.Unlock()
	b.compactMu.Unlock()
	if db == nil {
		return nil
	}

	if err := db.Close(); err != nil {
		return oops.Wrapf(err, "close badger result buffer")
	}
	return nil
}

func (b *badgerResultBuffer) operationError(ctx context.Context, op string) error {
	if b.closed {
		return oops.Wrapf(ErrResultBufferClosed, "%s", op)
	}
	if ctx == nil {
		return oops.Wrapf(ErrResultBufferContextRequired, "%s", op)
	}
	if err := ctx.Err(); err != nil {
		return oops.Wrapf(err, "%s", op)
	}
	return nil
}

func (b *badgerResultBuffer) trimOldest(ctx context.Context, deleteCount int) (bool, error) {
	if deleteCount <= 0 {
		return false, nil
	}

	keys, err := b.listKeys(ctx, deleteCount, "read badger result buffer trim keys")
	if err != nil {
		return false, oops.Wrapf(err, "trim badger result buffer")
	}
	if len(keys) == 0 {
		return false, nil
	}
	if err := b.namespace.DeleteMany(ctx, keys...); err != nil {
		return false, oops.Wrapf(err, "trim badger result buffer")
	}
	b.size = max(0, b.size-len(keys))
	return true, nil
}

func (b *badgerResultBuffer) listKeys(ctx context.Context, limit int, op string) ([]uint64, error) {
	entries, err := b.namespace.List(ctx, badgerx.WithLimit[uint64](limit))
	if err != nil {
		return nil, oops.Wrapf(err, "read badger result buffer keys: %s", op)
	}
	return collectionlist.MapList(
		collectionlist.NewList(entries...),
		func(_ int, entry badgerx.Entry[uint64, protocol.AgentResultRequest]) uint64 {
			return entry.Key
		},
	).Values(), nil
}

func (b *badgerResultBuffer) compactCandidate() (*badgerx.DB, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, false, oops.Wrapf(ErrResultBufferClosed, "badger result buffer compact")
	}
	if b.memory {
		return b.db, false, nil
	}
	if b.db == nil {
		return nil, false, oops.Wrapf(ErrResultBufferClosed, "badger result buffer compact")
	}

	now := time.Now()
	if b.compactAt.IsZero() {
		b.compactAt = now
	}
	if now.Before(b.compactAt) {
		return b.db, false, nil
	}
	if b.compactInterval <= 0 {
		return b.db, false, nil
	}

	b.compactAt = now.Add(b.compactInterval)
	return b.db, true, nil
}
