package monarch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL    = "https://api.monarchmoney.com"
	gqlURL     = baseURL + "/graphql"
	loginURL   = baseURL + "/auth/login/"
)

type Client struct {
	token  string
	http   *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Login authenticates with email/password and returns a token.
func Login(email, password string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"email":          email,
		"password":       password,
		"trusted_device": false,
		"totp":           nil,
	})

	resp, err := http.Post(loginURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed (status %d): %s", resp.StatusCode, string(raw))
	}

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}
	if lr.Token == "" {
		return "", fmt.Errorf("login succeeded but no token returned")
	}
	return lr.Token, nil
}

func query[T any](c *Client, q string, vars map[string]any) (T, error) {
	var zero T

	reqBody, err := json.Marshal(gqlRequest{Query: q, Variables: vars})
	if err != nil {
		return zero, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, gqlURL, bytes.NewReader(reqBody))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("graphql request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return zero, fmt.Errorf("graphql error (status %d): %s", resp.StatusCode, string(raw))
	}

	var result gqlResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(result.Errors) > 0 {
		return zero, fmt.Errorf("graphql error: %s", result.Errors[0].Message)
	}

	return result.Data, nil
}

func (c *Client) GetAccounts() (AccountsData, error) {
	const q = `query GetAccounts {
		accounts {
			id displayName currentBalance isHidden isAsset includeInNetWorth isManual mask logoUrl
			type { name display }
			subtype { name display }
			credential { id institution { name url } }
		}
	}`
	return query[AccountsData](c, q, nil)
}

func (c *Client) GetTransactions(startDate, endDate string, limit, offset int) (TransactionsData, error) {
	const q = `query GetTransactions($offset: Int, $limit: Int, $filters: TransactionFilterInput) {
		allTransactions(filters: $filters) {
			totalCount
			results(offset: $offset, limit: $limit) {
				id amount date pending notes isRecurring reviewStatus needsReview isSplitTransaction
				category { id name }
				merchant { id name }
				account { id displayName }
			}
		}
	}`
	filters := map[string]any{}
	if startDate != "" {
		filters["startDate"] = startDate
	}
	if endDate != "" {
		filters["endDate"] = endDate
	}
	if limit == 0 {
		limit = 100
	}
	return query[TransactionsData](c, q, map[string]any{
		"offset":  offset,
		"limit":   limit,
		"filters": filters,
	})
}

func (c *Client) GetCashflow(startDate, endDate string) (CashflowData, error) {
	const q = `query GetCashflow($filters: TransactionFilterInput) {
		summary: transactionsSummary(filters: $filters) {
			avg count max sum
			byCategory {
				groupBy {
					category {
						id name
						group { id name type }
					}
				}
				summary { sum count avg max }
			}
		}
	}`
	filters := map[string]any{}
	if startDate != "" {
		filters["startDate"] = startDate
	}
	if endDate != "" {
		filters["endDate"] = endDate
	}
	return query[CashflowData](c, q, map[string]any{"filters": filters})
}

func (c *Client) GetBudgets(startDate, endDate string) (BudgetsData, error) {
	const q = `query GetBudget($startDate: Date!, $endDate: Date!) {
		budget(startMonth: $startDate, endMonth: $endDate) {
			category { id name group { id name type } }
			totalBudgeted totalActual totalRemaining
			monthlyAmounts { month budgeted actual remaining }
		}
	}`
	return query[BudgetsData](c, q, map[string]any{
		"startDate": startDate,
		"endDate":   endDate,
	})
}

func (c *Client) GetNetWorth() (NetWorthData, error) {
	const q = `query GetNetWorth {
		netWorthSummary {
			netWorth
			breakdown { type balance percent }
		}
	}`
	return query[NetWorthData](c, q, nil)
}
