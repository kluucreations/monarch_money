package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func resultText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestParseInt(t *testing.T) {
	cases := []struct {
		input    string
		fallback int
		want     int
	}{
		{"42", 0, 42},
		{"0", 10, 0},
		{"-5", 0, -5},
		{"", 100, 100},
		{"abc", 7, 7},
		{"3.14", 1, 1},
	}
	for _, tc := range cases {
		got := parseInt(tc.input, tc.fallback)
		if got != tc.want {
			t.Errorf("parseInt(%q, %d) = %d, want %d", tc.input, tc.fallback, got, tc.want)
		}
	}
}

func TestJsonResult(t *testing.T) {
	type sample struct {
		Name  string  `json:"name"`
		Value float64 `json:"value"`
	}

	result, err := jsonResult(sample{Name: "test", Value: 1.23})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("got nil result")
	}

	var got sample
	if err := json.Unmarshal([]byte(resultText(result)), &got); err != nil {
		t.Fatalf("failed to unmarshal result text: %v", err)
	}
	if got.Name != "test" || got.Value != 1.23 {
		t.Errorf("got %+v, want {Name:test Value:1.23}", got)
	}
}

func TestJsonResultIsIndented(t *testing.T) {
	result, _ := jsonResult(map[string]any{"key": "val"})
	if !strings.Contains(resultText(result), "\n") {
		t.Error("expected indented JSON output")
	}
}

func TestClientFromContextMissingToken(t *testing.T) {
	_, err := clientFromContext(context.Background())
	if err == nil {
		t.Error("expected error when no token in context")
	}
}

func TestClientFromContextWithToken(t *testing.T) {
	ctx := context.WithValue(context.Background(), monarchTokenKey, "mytoken")
	c, err := clientFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Error("expected non-nil client")
	}
}

func TestClientFromContextEmptyToken(t *testing.T) {
	ctx := context.WithValue(context.Background(), monarchTokenKey, "")
	_, err := clientFromContext(ctx)
	if err == nil {
		t.Error("expected error for empty token")
	}
}

