// This example demonstrates a ReAct agent with tool usage.
// The agent uses a calculator tool and a weather lookup tool
// to answer user questions that require multi-step reasoning.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/vearne/agentscope-go/pkg/agent"
	"github.com/vearne/agentscope-go/pkg/formatter"
	"github.com/vearne/agentscope-go/pkg/memory"
	"github.com/vearne/agentscope-go/pkg/model"
	"github.com/vearne/agentscope-go/pkg/tool"
)

func main() {
	m := model.NewOpenAIChatModel("gpt-4o", "your-api-key", "", false)
	f := formatter.NewOpenAIChatFormatter()

	tk := tool.NewToolkit()
	if err := tool.RegisterShellTool(tk); err != nil {
		log.Fatal(err)
	}
	if err := registerWeatherTool(tk); err != nil {
		log.Fatal(err)
	}

	a := agent.NewReActAgent(
		agent.WithReActName("Friday"),
		agent.WithReActModel(m),
		agent.WithReActFormatter(f),
		agent.WithReActMemory(memory.NewInMemoryMemory()),
		agent.WithReActToolkit(tk),
		agent.WithReActMaxIters(5),
		agent.WithReActSystemPrompt(
			"You are a helpful assistant named Friday. Use tools when needed to answer questions accurately.",
		),
	)

	msg := agent.NewUserMsg("user", "What's the weather like in Beijing? And what is 123 * 456?")
	resp, err := a.Reply(context.Background(), msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("[%s] %s\n", resp.Name, resp.GetTextContent())
}

func registerWeatherTool(tk *tool.Toolkit) error {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{
				"type":        "string",
				"description": "The city to get weather for",
			},
		},
		"required": []string{"city"},
	}
	return tk.Register("get_weather", "Get the current weather for a city", params, getWeather)
}

func getWeather(_ context.Context, args map[string]interface{}) (*tool.ToolResponse, error) {
	city, _ := args["city"].(string)
	city = strings.ToLower(city)

	// Mock weather data
	weatherData := map[string]string{
		"beijing":  "Sunny, 25°C, humidity 45%",
		"shanghai": "Cloudy, 22°C, humidity 60%",
		"new york": "Rainy, 18°C, humidity 75%",
	}

	if weather, ok := weatherData[city]; ok {
		return &tool.ToolResponse{Content: weather}, nil
	}
	return &tool.ToolResponse{
		Content: fmt.Sprintf("Weather data not available for %s. It's probably nice weather though!", city),
	}, nil
}
