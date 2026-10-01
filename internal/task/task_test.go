package task

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestEventMessageJSONRoundTrip(t *testing.T) {
	want := EventMessage{
		Type: EventCreated,
		Task: Task{ID: uuid.Must(uuid.NewV7()), Title: "buy milk", Description: "2 litres"},
	}

	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	t.Log(string(b))

	var got EventMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
