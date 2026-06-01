package collector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/arcgolabs/storx/badgerx"
	"github.com/arcgolabs/storx/codec"
	"github.com/arcgolabs/storx/keycodec"
	"github.com/dgraph-io/badger/v4"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

const (
	resultBufferNamespace             = "agent-results"
	resultBufferCompactionInterval    = time.Minute
	resultBufferCompactionDiscardRate = 0.5
)

type badgerResultBuffer struct {
	mu                 sync.Mutex
	compactMu          sync.Mutex
	max                int
	size               int
	next               uint64
	db                 *badgerx.DB
	namespace          *badgerx.Namespace[uint64, protocol.AgentResultRequest]
	memory             bool
	compactAt          time.Time
	compactInterval    time.Duration
	compactDiscardRate float64
}

func NewPersistentResultBuffer(path string, capacity int) (*badgerResultBuffer, error) {
	return newPersistentResultBuffer(context.Background(), path, capacity)
}

func newPersistentResultBuffer(ctx context.Context, path string, capacity int) (*badgerResultBuffer, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: result buffer path is required", oops.New("invalid buffer config"))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return nil, oops.Wrapf(err, "create result buffer directory")
	}
	options := badger.DefaultOptions(path).WithLogger(nil)
	return newBadgerResultBuffer(ctx, options, path, capacity)
}

func NewMemoryBadgerResultBuffer(capacity int) (*badgerResultBuffer, error) {
	return newMemoryBadgerResultBuffer(context.Background(), capacity)
}

func newMemoryBadgerResultBuffer(ctx context.Context, capacity int) (*badgerResultBuffer, error) {
	options := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	return newBadgerResultBuffer(ctx, options, "", capacity)
}

func newBadgerResultBuffer(ctx context.Context, options badger.Options, path string, capacity int) (*badgerResultBuffer, error) {
	if capacity < 0 {
		capacity = 0
	}

	db, namespace, err := openResultBufferNamespace(ctx, options, path)
	if err != nil {
		return nil, oops.Wrapf(err, "open result buffer")
	}
	size, next, seedErr := seedBadgerResultBuffer(ctx, namespace)
	if seedErr == nil {
		return &badgerResultBuffer{
			max:                capacity,
			size:               size,
			next:               next,
			db:                 db,
			namespace:          namespace,
			memory:             options.InMemory,
			compactAt:          time.Now(),
			compactInterval:    resultBufferCompactionInterval,
			compactDiscardRate: resultBufferCompactionDiscardRate,
		}, nil
	}

	if path == "" {
		return nil, oops.Wrapf(seedErr, "seed badger result buffer")
	}
	recoveredDB, recoveredNamespace, recoveredSize, recoveredNext, recoveredErr := recoverResultBuffer(ctx, seedErr, db, options, path)
	if recoveredErr != nil {
		return nil, oops.Wrapf(recoveredErr, "recover badger result buffer")
	}

	return &badgerResultBuffer{
		max:                capacity,
		size:               recoveredSize,
		next:               recoveredNext,
		db:                 recoveredDB,
		namespace:          recoveredNamespace,
		memory:             options.InMemory,
		compactAt:          time.Now(),
		compactInterval:    resultBufferCompactionInterval,
		compactDiscardRate: resultBufferCompactionDiscardRate,
	}, nil
}

func recoverResultBuffer(
	ctx context.Context,
	recoverErr error,
	db *badgerx.DB,
	options badger.Options,
	path string,
) (*badgerx.DB, *badgerx.Namespace[uint64, protocol.AgentResultRequest], int, uint64, error) {
	if closeErr := db.Close(); closeErr != nil {
		return nil, nil, 0, 0, errors.Join(recoverErr, oops.Wrapf(closeErr, "close badger result buffer after seed error"))
	}
	if err := recoverResultBufferStorage(path, options.ValueDir); err != nil {
		return nil, nil, 0, 0, oops.Wrapf(err, "recover badger result buffer storage")
	}

	reopened, namespace, err := openResultBufferNamespace(ctx, options, path)
	if err != nil {
		return nil, nil, 0, 0, oops.Wrapf(err, "reopen result buffer after recovery")
	}
	size, next, err := seedBadgerResultBuffer(ctx, namespace)
	if err != nil {
		closeErr := reopened.Close()
		if closeErr != nil {
			return nil, nil, 0, 0, errors.Join(
				oops.Wrapf(err, "seed badger result buffer after recovery"),
				oops.Wrapf(closeErr, "close result buffer after recovery seed failure"),
			)
		}
		return nil, nil, 0, 0, oops.Wrapf(err, "seed badger result buffer after recovery")
	}

	return reopened, namespace, size, next, nil
}

func openResultBufferNamespace(ctx context.Context, options badger.Options, path string) (*badgerx.DB, *badgerx.Namespace[uint64, protocol.AgentResultRequest], error) {
	db, err := openBadgerWithRecovery(ctx, options, path)
	if err != nil {
		return nil, nil, oops.Wrapf(err, "open badger result buffer")
	}
	namespace := badgerx.NewNamespaceWithDB(
		db,
		resultBufferNamespace,
		keycodec.Uint64BE(),
		codec.JSON[protocol.AgentResultRequest](),
	)
	return db, namespace, nil
}

func openBadgerWithRecovery(ctx context.Context, options badger.Options, path string) (*badgerx.DB, error) {
	db, err := badgerx.Open(options)
	if err == nil {
		return db, nil
	}
	if path == "" || !errors.Is(err, badger.ErrTruncateNeeded) {
		return nil, oops.Wrapf(err, "open badger database")
	}
	if err := recoverResultBufferStorage(path, options.ValueDir); err != nil {
		return nil, oops.Wrapf(err, "recover badger result buffer storage")
	}
	if cancelErr := ctx.Err(); cancelErr != nil {
		return nil, oops.Wrapf(cancelErr, "open badger result buffer context canceled")
	}

	reopened, reopenErr := badgerx.Open(options)
	if reopenErr != nil {
		return nil, oops.Wrapf(reopenErr, "reopen badger result buffer after recovery")
	}
	return reopened, nil
}

func recoverResultBufferStorage(dataDir, valueDir string) error {
	if err := rotateBadgerPath(dataDir); err != nil {
		return err
	}
	if valueDir != "" && valueDir != dataDir {
		if err := rotateBadgerPath(valueDir); err != nil {
			return err
		}
	}
	return nil
}

func rotateBadgerPath(path string) error {
	if path == "" || path == "." {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return oops.Wrapf(err, "stat badger path")
	}
	backupPath := fmt.Sprintf("%s.corrupt.%s", path, time.Now().Format("20060102-150405.000000000"))
	if err := os.Rename(path, backupPath); err != nil {
		return oops.Wrapf(err, "rotate badger path")
	}
	return nil
}

func seedBadgerResultBuffer(
	ctx context.Context,
	namespace *badgerx.Namespace[uint64, protocol.AgentResultRequest],
) (int, uint64, error) {
	keys, err := namespace.Keys(ctx)
	if err != nil {
		return 0, 0, oops.Wrapf(err, "seed badger result buffer size")
	}

	next := uint64(1)
	tail, err := namespace.Keys(ctx, badgerx.WithReverse[uint64](true), badgerx.WithLimit[uint64](1))
	if err != nil {
		return 0, 0, oops.Wrapf(err, "seed badger result buffer tail")
	}
	if len(tail) > 0 {
		next = tail[0] + 1
	}
	return len(keys), next, nil
}
