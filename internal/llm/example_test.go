// Package llm provides an OpenAI-compatible HTTP client for interacting with LLM providers.
//
// # Supported Providers
//
// The client is compatible with any OpenAI-compatible API including:
//
//   - OpenAI (https://api.openai.com)
//   - OpenRouter (https://openrouter.ai/api)
//   - Ollama (http://localhost:11434/v1)
//
// # Quick Start
//
//	client, err := llm.NewClient(
//	    llm.WithAPIKey("sk-..."),
//	    llm.WithModel("gpt-4"),
//	)
//
// # Generate (Synchronous)
//
//	resp, err := client.Generate(ctx, llm.CompletionRequest{
//	    Model: "gpt-4",
//	    Messages: []llm.Message{
//	        {Role: llm.RoleSystem, Content: "You are a helpful assistant."},
//	        {Role: llm.RoleUser, Content: "Hello!"},
//	    },
//	})
//
// # Stream (Asynchronous)
//
//	ch, err := client.Stream(ctx, llm.CompletionRequest{
//	    Model: "gpt-4",
//	    Messages: []llm.Message{
//	        {Role: llm.RoleUser, Content: "Count to 5"},
//	    },
//	})
//	for chunk := range ch {
//	    fmt.Print(chunk.Choices[0].Delta.Content)
//	}
//
// # Embeddings
//
//	emb, err := client.Embed(ctx, llm.EmbeddingRequest{
//	    Model: "text-embedding-3-small",
//	    Input: "Hello world",
//	})
//
// # Tools/Function Calling
//
//	tools := []llm.Tool{
//	    {
//	        Type: "function",
//	        Function: llm.FunctionDefinition{
//	            Name:        "get_weather",
//	            Description: "Get the weather for a location",
//	            Parameters: map[string]interface{}{
//	                "type": "object",
//	                "properties": map[string]interface{}{
//	                    "location": map[string]string{"type": "string", "description": "The city name"},
//	                },
//	                "required": []string{"location"},
//	            },
//	            Strict: true,
//	        },
//	    },
//	}
//
//	resp, _ := client.Generate(ctx, llm.CompletionRequest{
//	    Model: "gpt-4",
//	    Messages: []llm.Message{
//	        {Role: llm.RoleUser, Content: "What's the weather in Tokyo?"},
//	    },
//	    Tools: tools,
//	})
//
// # With Metrics
//
//	tracedClient := client.WithMetrics()
//	resp, err := tracedClient.Generate(ctx, req)
package llm
