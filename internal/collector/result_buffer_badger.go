package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/storx/badgerx"
	"github.com/arcgolabs/storx/codec"
	"github.com/arcgolabs/storx/keycodec"
	"github.com/dgraph-io/badger/v4"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

const resultBufferNamespace = "agent-results"

type badgerResultBuffer struct {
	mu        sync.Mutex
	max       int
	db        *badgerx.DB
	namespace *badgerx.Namespace[uint64, protocol.AgentResultRequest]
}

func NewPersistentResultBuffer(path string, capacity int) (*badgerResultBuffer, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: result buffer path is required", oops.New("invalid buffer config"))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return nil, oops.Wrapf(err, "create result buffer directory")
	}
	options := badger.DefaultOptions(path).WithLogger(nil)
	return newBadgerResultBuffer(options, capacity)
}

func NewMemoryBadgerResultBuffer(capacity int) (*badgerResultBuffer, error) {
	options := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	return newBadgerResultBuffer(options, capacity)
}

func newBadgerResultBuffer(options badger.Options, capacity int) (*badgerResultBuffer, error) {
	if capacity < 0 {
		capacity = 0
	}
	db, err := badgerx.Open(options)
	if err != nil {
		return nil, oops.Wrapf(err, "open result buffer")
	}
	namespace := badgerx.NewNamespaceWithDB(
		db,
		resultBufferNamespace,
		keycodec.Uint64BE(),
		codec.JSON[protocol.AgentResultRequest](),
	)
	return &badgerResultBuffer{
		max:       capacity,
		db:        db,
		namespace: namespace,
	}, nil
}

func (b *badgerResultBuffer) Push(req protocol.AgentResultRequest) ResultQueuePush {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.max == 0 {
		return ResultQueuePush{}
	}

	ctx := context.Background()
	entries, err := b.namespace.List(ctx)
	if err != nil {
		return ResultQueuePush{err: oops.Wrapf(err, "list badger result buffer")}
	}

	deleteCount := max(0, len(entries)-b.max+1)
	if deleteCount > 0 {
		keys := make([]uint64, 0, deleteCount)
		for _, entry := range entries[:deleteCount] {
			keys = append(keys, entry.Key)
		}
		if err := b.namespace.DeleteMany(ctx, keys...); err != nil {
			return ResultQueuePush{err: oops.Wrapf(err, "trim badger result buffer")}
		}
	}

	key := nextBadgerResultKey(entries)
	if err := b.namespace.Set(ctx, key, req); err != nil {
		return ResultQueuePush{err: oops.Wrapf(err, "write badger result buffer")}
	}
	return ResultQueuePush{
		size:          len(entries) - deleteCount + 1,
		buffered:      true,
		droppedOldest: deleteCount > 0,
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
	return nil
}

func (b *badgerResultBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	keys, err := b.namespace.Keys(context.Background())
	if err != nil {
		return 0
	}
	return len(keys)
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

func nextBadgerResultKey(entries []badgerx.Entry[uint64, protocol.AgentResultRequest]) uint64 {
	if len(entries) == 0 {
		return 1
	}
	return entries[len(entries)-1].Key + 1
}
