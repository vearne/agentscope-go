package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/message"
)

func main() {
	ctx := context.Background()

	// Example 1: InMemoryMemory with marks
	fmt.Println("=== InMemoryMemory Example ===")
	inMem := memory.NewInMemoryMemory()

	msg1 := message.NewMsg("user", "Hello, I'm looking for a job", "user")
	msg2 := message.NewMsg("assistant", "I can help you with that", "assistant")
	msg3 := message.NewMsg("user", "I'm a software engineer", "user")

	if err := inMem.AddWithMarks(ctx, []*message.Msg{msg1, msg3}, []string{"job-search"}); err != nil {
		log.Fatal(err)
	}
	if err := inMem.AddWithMarks(ctx, []*message.Msg{msg2}, []string{"assistant-response"}); err != nil {
		log.Fatal(err)
	}

	allMsgs := inMem.GetMessages()
	fmt.Printf("Total messages: %d\n", len(allMsgs))

	jobMsgs, _ := inMem.GetMemory(ctx, "job-search", "", false)
	fmt.Printf("Job search messages: %d\n", len(jobMsgs))

	if _, err := inMem.UpdateMessagesMark(ctx, "important", "job-search", nil); err != nil {
		log.Fatal(err)
	}

	importantMsgs, _ := inMem.GetMemory(ctx, "important", "", false)
	fmt.Printf("Important messages: %d\n", len(importantMsgs))

	// Example 2: Mem0LongTermMemory
	fmt.Println("\n=== Mem0LongTermMemory Example ===")
	ltm := memory.NewMem0LongTermMemory(
		memory.WithAgentID("career-assistant"),
		memory.WithMem0UserID("user-123"),
	)

	recordMsg := message.NewMsg("user", "John is a software engineer with 5 years experience", "user")
	_, err := ltm.Record(ctx, []*message.Msg{recordMsg})
	if err != nil {
		log.Printf("Error recording: %v", err)
	}

	response, err := ltm.RecordToMemory(ctx, "User's professional background", []string{
		"Software engineer",
		"5 years experience",
		"Looking for senior role",
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Recorded: %s\n", response.Content)
	}

	searchResponse, err := ltm.RetrieveFromMemory(ctx, []string{"software engineer", "experience"}, 5)
	if err != nil {
		log.Printf("Error retrieving: %v", err)
	} else {
		fmt.Printf("Retrieved: %s\n", searchResponse.Content)
	}

	fmt.Printf("Total memories: %d\n", ltm.GetMemoryCount())

	// Example 3: Using marks for conversation management
	fmt.Println("\n=== Mark Management Example ===")
	mem := memory.NewInMemoryMemory()

	welcomeMsg := message.NewMsg("user", "Hi, I need help", "user")
	questionMsg := message.NewMsg("user", "How do I implement caching?", "user")
	answerMsg := message.NewMsg("assistant", "You can use Redis for caching", "assistant")

	if err := mem.AddWithMarks(ctx, []*message.Msg{welcomeMsg}, []string{"greeting"}); err != nil {
		log.Fatal(err)
	}
	if err := mem.AddWithMarks(ctx, []*message.Msg{questionMsg, answerMsg}, []string{"technical", "caching"}); err != nil {
		log.Fatal(err)
	}

	technicalMsgs, _ := mem.GetMemory(ctx, "technical", "", false)
	fmt.Printf("Technical messages: %d\n", len(technicalMsgs))

	nonGreetingMsgs, _ := mem.GetMemory(ctx, "", "greeting", false)
	fmt.Printf("Non-greeting messages: %d\n", len(nonGreetingMsgs))

	if err := mem.UpdateCompressedSummary(ctx, "User is asking about caching implementation"); err != nil {
		log.Fatal(err)
	}

	msgsWithSummary, _ := mem.GetMemory(ctx, "", "", true)
	fmt.Printf("Messages with summary: %d (first is summary)\n", len(msgsWithSummary))

	fmt.Println("\nAll examples completed successfully!")
}
