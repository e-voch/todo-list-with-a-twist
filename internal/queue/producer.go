package queue

import (
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"todo/internal/task"
)

type Producer struct {
	p     *kafka.Producer
	topic string
}

func NewProducer(brokers, topic string) (*Producer, error) {
	kp, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		return nil, fmt.Errorf("create producer: %w", err)
	}
	return &Producer{p: kp, topic: topic}, nil
}

// Publish sends e to the topic and waits until Kafka confirms it was stored.
func (p *Producer) Publish(e task.EventMessage) error {
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	delivery := make(chan kafka.Event, 1)
	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &p.topic, Partition: kafka.PartitionAny},
		Value:          b,
	}
	if err := p.p.Produce(msg, delivery); err != nil {
		return fmt.Errorf("produce event: %w", err)
	}

	ev := <-delivery
	if err := ev.(*kafka.Message).TopicPartition.Error; err != nil {
		return fmt.Errorf("deliver event: %w", err)
	}
	return nil
}

func (p *Producer) Close() {
	p.p.Flush(5000)
	p.p.Close()
}
