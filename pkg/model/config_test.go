package model

import (
	"testing"
)

func TestModelConfigOptions(t *testing.T) {
	tests := []struct {
		name string
		setup func() *OpenAIChatModel
		wantTemp     *float64
		wantTopP     *float64
		wantMaxTokens *int
	}{
		{
			name: "WithTemperature",
			setup: func() *OpenAIChatModel {
				return NewOpenAIChatModel("gpt-4", "test-key", "", false,
					WithTemperature(0.7))
			},
			wantTemp: func() *float64 { v := 0.7; return &v }(),
		},
		{
			name: "WithTopP",
			setup: func() *OpenAIChatModel {
				return NewOpenAIChatModel("gpt-4", "test-key", "", false,
					WithTopP(0.9))
			},
			wantTopP: func() *float64 { v := 0.9; return &v }(),
		},
		{
			name: "WithMaxTokens",
			setup: func() *OpenAIChatModel {
				return NewOpenAIChatModel("gpt-4", "test-key", "", false,
					WithMaxTokens(2048))
			},
			wantMaxTokens: func() *int { v := 2048; return &v }(),
		},
		{
			name: "MultipleOptions",
			setup: func() *OpenAIChatModel {
				return NewOpenAIChatModel("gpt-4", "test-key", "", false,
					WithTemperature(0.5),
					WithTopP(0.8),
					WithMaxTokens(4096))
			},
			wantTemp:      func() *float64 { v := 0.5; return &v }(),
			wantTopP:      func() *float64 { v := 0.8; return &v }(),
			wantMaxTokens: func() *int { v := 4096; return &v }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			if m.config == nil {
				t.Fatal("config should not be nil")
			}

			if tt.wantTemp != nil {
				if m.config.Temperature == nil {
					t.Errorf("Temperature should not be nil")
				} else if *m.config.Temperature != *tt.wantTemp {
					t.Errorf("Temperature = %v, want %v", *m.config.Temperature, *tt.wantTemp)
				}
			}

			if tt.wantTopP != nil {
				if m.config.TopP == nil {
					t.Errorf("TopP should not be nil")
				} else if *m.config.TopP != *tt.wantTopP {
					t.Errorf("TopP = %v, want %v", *m.config.TopP, *tt.wantTopP)
				}
			}

			if tt.wantMaxTokens != nil {
				if m.config.MaxTokens == nil {
					t.Errorf("MaxTokens should not be nil")
				} else if *m.config.MaxTokens != *tt.wantMaxTokens {
					t.Errorf("MaxTokens = %v, want %v", *m.config.MaxTokens, *tt.wantMaxTokens)
				}
			}
		})
	}
}

func TestAnthropicModelConfigOptions(t *testing.T) {
	tests := []struct {
		name string
		setup func() *AnthropicChatModel
		wantTemp *float64
		wantTopP *float64
		wantTopK *int
	}{
		{
			name: "WithAnthropicTemperature",
			setup: func() *AnthropicChatModel {
				return NewAnthropicChatModel("claude-3", "test-key", "", false,
					WithAnthropicTemperature(0.7))
			},
			wantTemp: func() *float64 { v := 0.7; return &v }(),
		},
		{
			name: "WithAnthropicTopK",
			setup: func() *AnthropicChatModel {
				return NewAnthropicChatModel("claude-3", "test-key", "", false,
					WithAnthropicTopK(40))
			},
			wantTopK: func() *int { v := 40; return &v }(),
		},
		{
			name: "MultipleAnthropicOptions",
			setup: func() *AnthropicChatModel {
				return NewAnthropicChatModel("claude-3", "test-key", "", false,
					WithAnthropicTemperature(0.5),
					WithAnthropicTopP(0.9),
					WithAnthropicTopK(50))
			},
			wantTemp: func() *float64 { v := 0.5; return &v }(),
			wantTopP: func() *float64 { v := 0.9; return &v }(),
			wantTopK: func() *int { v := 50; return &v }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			if m.config == nil {
				t.Fatal("config should not be nil")
			}

			if tt.wantTemp != nil {
				if m.config.Temperature == nil {
					t.Errorf("Temperature should not be nil")
				} else if *m.config.Temperature != *tt.wantTemp {
					t.Errorf("Temperature = %v, want %v", *m.config.Temperature, *tt.wantTemp)
				}
			}

			if tt.wantTopP != nil {
				if m.config.TopP == nil {
					t.Errorf("TopP should not be nil")
				} else if *m.config.TopP != *tt.wantTopP {
					t.Errorf("TopP = %v, want %v", *m.config.TopP, *tt.wantTopP)
				}
			}

			if tt.wantTopK != nil {
				if m.config.TopK == nil {
					t.Errorf("TopK should not be nil")
				} else if *m.config.TopK != *tt.wantTopK {
					t.Errorf("TopK = %v, want %v", *m.config.TopK, *tt.wantTopK)
				}
			}
		})
	}
}

func TestGeminiModelConfigOptions(t *testing.T) {
	tests := []struct {
		name string
		setup func() *GeminiChatModel
		wantTemp *float64
		wantTopP *float64
		wantTopK *int
	}{
		{
			name: "WithGeminiTemperature",
			setup: func() *GeminiChatModel {
				return NewGeminiChatModel("gemini-pro", "test-key", "", false,
					WithGeminiTemperature(0.7))
			},
			wantTemp: func() *float64 { v := 0.7; return &v }(),
		},
		{
			name: "WithGeminiTopK",
			setup: func() *GeminiChatModel {
				return NewGeminiChatModel("gemini-pro", "test-key", "", false,
					WithGeminiTopK(40))
			},
			wantTopK: func() *int { v := 40; return &v }(),
		},
		{
			name: "MultipleGeminiOptions",
			setup: func() *GeminiChatModel {
				return NewGeminiChatModel("gemini-pro", "test-key", "", false,
					WithGeminiTemperature(0.5),
					WithGeminiTopP(0.9),
					WithGeminiTopK(50))
			},
			wantTemp: func() *float64 { v := 0.5; return &v }(),
			wantTopP: func() *float64 { v := 0.9; return &v }(),
			wantTopK: func() *int { v := 50; return &v }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			if m.config == nil {
				t.Fatal("config should not be nil")
			}

			if tt.wantTemp != nil {
				if m.config.Temperature == nil {
					t.Errorf("Temperature should not be nil")
				} else if *m.config.Temperature != *tt.wantTemp {
					t.Errorf("Temperature = %v, want %v", *m.config.Temperature, *tt.wantTemp)
				}
			}

			if tt.wantTopP != nil {
				if m.config.TopP == nil {
					t.Errorf("TopP should not be nil")
				} else if *m.config.TopP != *tt.wantTopP {
					t.Errorf("TopP = %v, want %v", *m.config.TopP, *tt.wantTopP)
				}
			}

			if tt.wantTopK != nil {
				if m.config.TopK == nil {
					t.Errorf("TopK should not be nil")
				} else if *m.config.TopK != *tt.wantTopK {
					t.Errorf("TopK = %v, want %v", *m.config.TopK, *tt.wantTopK)
				}
			}
		})
	}
}

func TestConfigValueFunctions(t *testing.T) {
	t.Run("configValueFloat64 - optVal takes precedence", func(t *testing.T) {
		defaultVal := func() *float64 { v := 0.5; return &v }()
		optVal := func() *float64 { v := 0.8; return &v }()
		result := configValueFloat64(defaultVal, optVal)
		if *result != 0.8 {
			t.Errorf("Expected 0.8, got %v", *result)
		}
	})

	t.Run("configValueFloat64 - defaultVal when optVal is nil", func(t *testing.T) {
		defaultVal := func() *float64 { v := 0.5; return &v }()
		result := configValueFloat64(defaultVal, nil)
		if *result != 0.5 {
			t.Errorf("Expected 0.5, got %v", *result)
		}
	})

	t.Run("configValueInt - optVal takes precedence", func(t *testing.T) {
		defaultVal := func() *int { v := 100; return &v }()
		optVal := func() *int { v := 200; return &v }()
		result := configValueInt(defaultVal, optVal)
		if *result != 200 {
			t.Errorf("Expected 200, got %v", *result)
		}
	})

	t.Run("configValueInt - defaultVal when optVal is nil", func(t *testing.T) {
		defaultVal := func() *int { v := 100; return &v }()
		result := configValueInt(defaultVal, nil)
		if *result != 100 {
			t.Errorf("Expected 100, got %v", *result)
		}
	})
}
