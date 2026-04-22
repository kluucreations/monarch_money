package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/kluu/monarch-mcp/monarch"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	client, err := buildClient()
	if err != nil {
		log.Fatalf("failed to initialize Monarch Money client: %v", err)
	}

	s := server.NewMCPServer(
		"monarch-money",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	registerTools(s, client)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	baseURL := fmt.Sprintf("http://0.0.0.0:%s", port)
	log.Printf("monarch-money MCP server started, serving on %s", baseURL)

	sse := server.NewSSEServer(s, server.WithBaseURL(baseURL))
	if err := sse.Start(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func buildClient() (*monarch.Client, error) {
	if token := os.Getenv("MONARCH_TOKEN"); token != "" {
		return monarch.NewClient(token), nil
	}
	email := os.Getenv("MONARCH_EMAIL")
	password := os.Getenv("MONARCH_PASSWORD")
	if email == "" || password == "" {
		return nil, fmt.Errorf("set MONARCH_TOKEN or both MONARCH_EMAIL and MONARCH_PASSWORD")
	}
	token, err := monarch.Login(email, password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	return monarch.NewClient(token), nil
}

func registerTools(s *server.MCPServer, c *monarch.Client) {
	// get_accounts
	s.AddTool(
		mcp.NewTool("get_accounts",
			mcp.WithDescription("List all financial accounts linked to Monarch Money, including balances and account types."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := c.GetAccounts()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(data.Accounts)
		},
	)

	// get_transactions
	s.AddTool(
		mcp.NewTool("get_transactions",
			mcp.WithDescription("Fetch transactions with optional date range and pagination. Dates must be YYYY-MM-DD."),
			mcp.WithString("start_date", mcp.Description("Start date (YYYY-MM-DD), inclusive")),
			mcp.WithString("end_date", mcp.Description("End date (YYYY-MM-DD), inclusive")),
			mcp.WithString("limit", mcp.Description("Maximum number of transactions to return (default 100)")),
			mcp.WithString("offset", mcp.Description("Number of transactions to skip for pagination (default 0)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			start := req.GetString("start_date", "")
			end := req.GetString("end_date", "")
			limit := parseInt(req.GetString("limit", "100"), 100)
			offset := parseInt(req.GetString("offset", "0"), 0)

			data, err := c.GetTransactions(start, end, limit, offset)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"total_count":  data.AllTransactions.TotalCount,
				"transactions": data.AllTransactions.Results,
			})
		},
	)

	// get_cashflow
	s.AddTool(
		mcp.NewTool("get_cashflow",
			mcp.WithDescription("Get income/expense summary and per-category breakdown for a date range. Dates must be YYYY-MM-DD."),
			mcp.WithString("start_date", mcp.Description("Start date (YYYY-MM-DD)")),
			mcp.WithString("end_date", mcp.Description("End date (YYYY-MM-DD)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			start := req.GetString("start_date", "")
			end := req.GetString("end_date", "")

			data, err := c.GetCashflow(start, end)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(data.Summary)
		},
	)

	// get_budgets
	s.AddTool(
		mcp.NewTool("get_budgets",
			mcp.WithDescription("Get budget vs. actual spending by category for a date range. Dates must be YYYY-MM-DD."),
			mcp.WithString("start_date",
				mcp.Description("Start month (YYYY-MM-DD, first day of month)"),
				mcp.Required(),
			),
			mcp.WithString("end_date",
				mcp.Description("End month (YYYY-MM-DD, first day of month)"),
				mcp.Required(),
			),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			start := req.GetString("start_date", "")
			end := req.GetString("end_date", "")

			data, err := c.GetBudgets(start, end)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(data.Budget)
		},
	)

	// get_net_worth
	s.AddTool(
		mcp.NewTool("get_net_worth",
			mcp.WithDescription("Get current net worth and asset/liability breakdown."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, err := c.GetNetWorth()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(data.NetWorthSummary)
		},
	)
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func parseInt(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}
