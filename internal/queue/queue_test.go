package queue

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"

	"todo/internal/task"
)

// brokers is the address of the Kafka container shared by every test in this
// package. It stays empty when tests run with -short.
var brokers string

// TestMain starts one Kafka container for the whole package, because starting
// it takes several seconds.
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()
	container, err := tckafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "start kafka container:", err)
		os.Exit(1)
	}

	addrs, err := container.Brokers(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "get kafka brokers:", err)
		testcontainers.TerminateContainer(container)
		os.Exit(1)
	}
	brokers = strings.Join(addrs, ",")

	code := m.Run()
	testcontainers.TerminateContainer(container)
	os.Exit(code)
}

func skipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("needs Docker; skipped with -short")
	}
}

// newTopic creates a single-partition topic with a random name, so tests
// never see each other's messages.
func newTopic(t *testing.T) string {
	t.Helper()
	admin, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		t.Fatalf("create admin client: %v", err)
	}
	defer admin.Close()

	topic := "test-" + uuid.NewString()
	results, err := admin.CreateTopics(context.Background(), []kafka.TopicSpecification{
		{Topic: topic, NumPartitions: 1, ReplicationFactor: 1},
	})
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if err := results[0].Error; err.Code() != kafka.ErrNoError {
		t.Fatalf("create topic %s: %v", topic, err)
	}
	return topic
}

func newProducer(t *testing.T, topic string) *Producer {
	t.Helper()
	p, err := NewProducer(brokers, topic)
	if err != nil {
		t.Fatalf("NewProducer: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

// newConsumer returns a consumer the caller must Close, because some tests
// close it early to simulate a restart.
func newConsumer(t *testing.T, topic, group string) *Consumer {
	t.Helper()
	c, err := NewConsumer(brokers, group, topic)
	if err != nil {
		t.Fatalf("NewConsumer: %v", err)
	}
	return c
}

func newEvent(title string) task.EventMessage {
	return task.EventMessage{
		Type: task.EventCreated,
		Task: task.Task{ID: uuid.Must(uuid.NewV7()), Title: title, Description: "Description"},
	}
}

// readNext calls Read until a message or an error arrives. The first reads
// return nothing while the consumer joins its group, so a single Read is
// not enough.
func readNext(t *testing.T, c *Consumer) (*Message, error) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := c.Read(500 * time.Millisecond)
		if err != nil || msg != nil {
			return msg, err
		}
	}
	t.Fatal("no message within 30s")
	return nil, nil
}

func readEvent(t *testing.T, c *Consumer) *Message {
	t.Helper()
	msg, err := readNext(t, c)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	return msg
}

func Test_PublishAndRead(t *testing.T) {
	skipIfShort(t)
	topic := newTopic(t)
	p := newProducer(t, topic)
	c := newConsumer(t, topic, uuid.NewString())
	defer c.Close()

	want := newEvent("Buy")
	if err := p.Publish(want); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	got := readEvent(t, c)
	if got.Event != want {
		t.Errorf("read event = %v, want %v", got.Event, want)
	}
}

func Test_ReadKeepsOrder(t *testing.T) {
	skipIfShort(t)
	topic := newTopic(t)
	p := newProducer(t, topic)
	c := newConsumer(t, topic, uuid.NewString())
	defer c.Close()

	var want []task.EventMessage
	for i := range 5 {
		e := newEvent(fmt.Sprintf("Task %d", i))
		if err := p.Publish(e); err != nil {
			t.Fatalf("Publish returned error: %v", err)
		}
		want = append(want, e)
	}

	for i, w := range want {
		got := readEvent(t, c)
		if got.Event != w {
			t.Errorf("event %d = %v, want %v", i, got.Event, w)
		}
	}
}

// An event that was read but not committed must be delivered again after a
// restart. The worker relies on this to never lose an event.
func Test_UncommittedIsRedelivered(t *testing.T) {
	skipIfShort(t)
	topic := newTopic(t)
	group := uuid.NewString()
	p := newProducer(t, topic)

	want := newEvent("Buy")
	if err := p.Publish(want); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	first := newConsumer(t, topic, group)
	if got := readEvent(t, first); got.Event != want {
		t.Fatalf("first read = %v, want %v", got.Event, want)
	}
	first.Close() // "crash" before Commit

	second := newConsumer(t, topic, group)
	defer second.Close()
	if got := readEvent(t, second); got.Event != want {
		t.Errorf("after restart without commit, read = %v, want the same event %v", got.Event, want)
	}
}

// A committed event must not be delivered again after a restart.
func Test_CommittedIsNotRedelivered(t *testing.T) {
	skipIfShort(t)
	topic := newTopic(t)
	group := uuid.NewString()
	p := newProducer(t, topic)

	done, next := newEvent("Done"), newEvent("Next")
	for _, e := range []task.EventMessage{done, next} {
		if err := p.Publish(e); err != nil {
			t.Fatalf("Publish returned error: %v", err)
		}
	}

	first := newConsumer(t, topic, group)
	msg := readEvent(t, first)
	if msg.Event != done {
		t.Fatalf("first read = %v, want %v", msg.Event, done)
	}
	if err := first.Commit(msg); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}
	first.Close()

	second := newConsumer(t, topic, group)
	defer second.Close()
	if got := readEvent(t, second); got.Event != next {
		t.Errorf("after commit and restart, read = %v, want the next event %v", got.Event, next)
	}
}

func Test_ReadTimeout(t *testing.T) {
	skipIfShort(t)
	c := newConsumer(t, newTopic(t), uuid.NewString())
	defer c.Close()

	msg, err := c.Read(2 * time.Second)
	if err != nil {
		t.Errorf("Read on empty topic returned error %v, want nil", err)
	}
	if msg != nil {
		t.Errorf("Read on empty topic returned %v, want nil", msg)
	}
}

func Test_ReadBadMessage(t *testing.T) {
	skipIfShort(t)
	topic := newTopic(t)

	// Our Producer only sends valid JSON, so use the raw Kafka producer.
	raw, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		t.Fatalf("create raw producer: %v", err)
	}
	defer raw.Close()
	delivery := make(chan kafka.Event, 1)
	err = raw.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          []byte("not json"),
	}, delivery)
	if err != nil {
		t.Fatalf("produce raw message: %v", err)
	}
	if err := (<-delivery).(*kafka.Message).TopicPartition.Error; err != nil {
		t.Fatalf("deliver raw message: %v", err)
	}

	c := newConsumer(t, topic, uuid.NewString())
	defer c.Close()
	if _, err := readNext(t, c); err == nil {
		t.Error("Read of a non-JSON message returned nil error, want a decode error")
	}
}
