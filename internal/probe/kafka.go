package probe

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/segmentio/kafka-go"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const defaultKafkaPort = "9092"

type kafkaProbeTarget struct {
	address string
	topic   string
}

func (c *Checker) checkKafka(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target, detail, err := parseKafkaProbeTarget(task.Target)
	if err != nil {
		return model.StatusDown, detail, err
	}

	conn, err := dialKafkaProbe(ctx, task, target)
	if err != nil {
		return model.StatusDown, detail, err
	}
	defer closeSilently(conn)

	if deadline, ok := ctx.Deadline(); ok {
		if deadlineErr := conn.SetDeadline(deadline); deadlineErr != nil {
			return model.StatusDown, detail, wrapError(deadlineErr, "set Kafka probe deadline")
		}
	}

	if err := readKafkaMetadata(conn, target, detail); err != nil {
		return model.StatusDown, detail, err
	}
	return model.StatusUp, detail, nil
}

func dialKafkaProbe(ctx context.Context, task protocol.AgentTask, target kafkaProbeTarget) (*kafka.Conn, error) {
	conn, err := (&kafka.Dialer{
		Timeout:  taskTimeout(task),
		ClientID: "orivis-agent",
	}).DialContext(ctx, "tcp", target.address)
	if err != nil {
		return nil, wrapError(err, "dial Kafka broker")
	}
	return conn, nil
}

func readKafkaMetadata(conn *kafka.Conn, target kafkaProbeTarget, detail map[string]any) error {
	brokers, err := conn.Brokers()
	if err != nil {
		return wrapError(err, "read Kafka broker metadata")
	}
	if len(brokers) == 0 {
		return errorf("Kafka metadata returned no brokers")
	}
	detail["broker_count"] = len(brokers)
	detail["brokers"] = brokers

	controller, err := conn.Controller()
	if err != nil {
		return wrapError(err, "read Kafka controller metadata")
	}
	detail["controller"] = controller

	if target.topic != "" {
		return readKafkaTopicMetadata(conn, target, detail)
	}
	return nil
}

func readKafkaTopicMetadata(conn *kafka.Conn, target kafkaProbeTarget, detail map[string]any) error {
	partitions, err := conn.ReadPartitions(target.topic)
	if err != nil {
		return wrapError(err, "read Kafka topic metadata")
	}
	detail["topic"] = target.topic
	detail["partition_count"] = len(partitions)
	return nil
}

func parseKafkaProbeTarget(raw string) (kafkaProbeTarget, map[string]any, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return kafkaProbeTarget{}, map[string]any{"target": raw}, errorf("Kafka probe target is empty")
	}
	if strings.Contains(target, "://") || strings.Contains(target, "?") {
		return parseKafkaURLTarget(raw, target)
	}

	address := ensureHostPort(target, defaultKafkaPort)
	return kafkaProbeTarget{address: address}, map[string]any{"target": address}, nil
}

func parseKafkaURLTarget(raw, target string) (kafkaProbeTarget, map[string]any, error) {
	if !strings.Contains(target, "://") {
		target = "kafka://" + target
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return kafkaProbeTarget{}, map[string]any{"target": raw}, wrapError(err, "parse Kafka probe target")
	}
	if !strings.EqualFold(parsed.Scheme, string(model.MonitorKafka)) {
		return kafkaProbeTarget{}, map[string]any{"target": raw, "scheme": parsed.Scheme}, errorf("unsupported Kafka target scheme %q", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return kafkaProbeTarget{}, map[string]any{"target": raw}, errorf("Kafka probe target host is empty")
	}

	topic := strings.TrimSpace(parsed.Query().Get("topic"))
	if topic == "" {
		topic = strings.Trim(strings.TrimSpace(parsed.Path), "/")
	}

	address := net.JoinHostPort(host, firstNonEmpty(parsed.Port(), defaultKafkaPort))
	detail := map[string]any{"target": address}
	if topic != "" {
		detail["topic"] = topic
	}
	return kafkaProbeTarget{address: address, topic: topic}, detail, nil
}
