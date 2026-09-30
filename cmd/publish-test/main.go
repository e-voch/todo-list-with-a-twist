package main

import (
	"fmt"
	"todo/internal/queue"
	"todo/internal/task"
)

func main() {
	testProducer, err := queue.NewProducer("localhost:9092", "tasks")
	if err != nil {
		fmt.Println("create failed:", err)
		return
	}
	defer testProducer.Close()

	event := task.EventMessage{
		Type: task.EventCreated,
		Task: task.Task{Title: "sell milk", Description: "10 litres"},
	}

	if err := testProducer.Publish(event); err != nil {
		fmt.Println("publish failed:", err)
		return
	}
	fmt.Println("published!")
}