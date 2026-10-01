package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"todo/internal/task"
)

type Consumer struct {
	c *kafka.Consumer
}

type Message struct {
	Event task.EventMessage
	msg   *kafka.Message
}

func NewConsumer(brokers, groupID, topic string) (*Consumer, error) {
	kc, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          groupID,
		// Start from the oldest message the first time this group connects.
		"auto.offset.reset": "earliest",
		// Offsets are committed by hand, only after an event has been handled.
		"enable.auto.commit": false,
	})
	if err != nil {
		return nil, fmt.Errorf("create consumer: %w", err)
	}

	if err := kc.SubscribeTopics([]string{topic}, nil); err != nil {
		kc.Close()
		return nil, fmt.Errorf("subscribe to %q: %w", topic, err)
	}
	return &Consumer{c: kc}, nil
}

// Read waits up to timeout for the next event. It returns nil, nil if no
// event arrived in time.
func (c *Consumer) Read(timeout time.Duration) (*Message, error) {
	km, err := c.c.ReadMessage(timeout)
	if err != nil {
		var kerr kafka.Error
		if errors.As(err, &kerr) && kerr.IsTimeout() {
			return nil, nil
		}
		return nil, fmt.Errorf("read message: %w", err)
	}

	var e task.EventMessage
	if err := json.Unmarshal(km.Value, &e); err != nil {
		return nil, fmt.Errorf("decode message at offset %v: %w", km.TopicPartition.Offset, err)
	}
	return &Message{Event: e, msg: km}, nil
}

// Commit tells Kafka that m has been handled, so the group will not be given
// it again after a restart.
func (c *Consumer) Commit(m *Message) error {
	if _, err := c.c.CommitMessage(m.msg); err != nil {
		return fmt.Errorf("commit offset %v: %w", m.msg.TopicPartition.Offset, err)
	}
	return nil
}

func (c *Consumer) Close() error {
	return c.c.Close()
}
