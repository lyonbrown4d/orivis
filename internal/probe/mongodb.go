package probe

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const defaultMongoDBPort = "27017"

func (c *Checker) checkMongoDB(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target, detail, err := mongoDBProbeTarget(task.Target)
	if err != nil {
		return model.StatusDown, detail, err
	}

	timeout := taskTimeout(task)
	client, err := mongo.Connect(options.Client().ApplyURI(target).SetServerSelectionTimeout(timeout))
	if err != nil {
		return model.StatusDown, detail, wrapError(err, "create MongoDB probe client")
	}
	defer func() {
		if disconnectErr := client.Disconnect(context.WithoutCancel(ctx)); disconnectErr != nil {
			return
		}
	}()

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return model.StatusDown, detail, wrapError(err, "execute MongoDB probe")
	}
	return model.StatusUp, detail, nil
}

func mongoDBProbeTarget(raw string) (string, map[string]any, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", map[string]any{"target": raw}, errorf("MongoDB probe target is empty")
	}

	lowerTarget := strings.ToLower(target)
	if !strings.HasPrefix(lowerTarget, "mongodb://") && !strings.HasPrefix(lowerTarget, "mongodb+srv://") {
		target = "mongodb://" + ensureHostPort(target, defaultMongoDBPort)
	}
	return target, map[string]any{"target": target}, nil
}
