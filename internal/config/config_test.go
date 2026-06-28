package config

import (
	"os"
	"testing"
)

func TestGetEnvOrDefault(t *testing.T) {
	// Тест: env переменная не задана, используется default
	result := getEnvOrDefault("NON_EXISTENT_VAR_123", "default_value")
	if result != "default_value" {
		t.Errorf("expected 'default_value', got '%s'", result)
	}

	// Тест: env переменная задана
	os.Setenv("TEST_ENV_VAR", "env_value")
	defer os.Unsetenv("TEST_ENV_VAR")

	result = getEnvOrDefault("TEST_ENV_VAR", "default_value")
	if result != "env_value" {
		t.Errorf("expected 'env_value', got '%s'", result)
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"300", 300},
		{"-5", -5},
		{"1000", 1000},
	}

	for _, tt := range tests {
		result := parseInt(tt.input)
		if result != tt.expected {
			t.Errorf("parseInt(%s) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestParseIntInvalid(t *testing.T) {
	// parseInt логирует ошибку и возвращает 0 при невалидном вводе
	result := parseInt("invalid")
	if result != 0 {
		t.Errorf("parseInt('invalid') = %d, want 0", result)
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"True", true},
		{"1", true},
		{"0", false},
	}

	for _, tt := range tests {
		result := parseBool(tt.input)
		if result != tt.expected {
			t.Errorf("parseBool(%s) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
