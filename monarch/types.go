package monarch

// GraphQL request/response envelope

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []gqlError     `json:"errors,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
}

// Auth

type loginResponse struct {
	Token string `json:"token"`
}

// Accounts

type AccountsData struct {
	Accounts []Account `json:"accounts"`
}

type Account struct {
	ID              string          `json:"id"`
	DisplayName     string          `json:"displayName"`
	CurrentBalance  float64         `json:"currentBalance"`
	IsHidden        bool            `json:"isHidden"`
	IsAsset         bool            `json:"isAsset"`
	IncludeInNetWorth bool          `json:"includeInNetWorth"`
	IsManual        bool            `json:"isManual"`
	Mask            string          `json:"mask"`
	LogoURL         string          `json:"logoUrl"`
	Type            AccountType     `json:"type"`
	Subtype         AccountSubtype  `json:"subtype"`
	Credential      *Credential     `json:"credential"`
}

type AccountType struct {
	Name    string `json:"name"`
	Display string `json:"display"`
}

type AccountSubtype struct {
	Name    string `json:"name"`
	Display string `json:"display"`
}

type Credential struct {
	ID          string      `json:"id"`
	Institution Institution `json:"institution"`
}

type Institution struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Transactions

type TransactionsData struct {
	AllTransactions TransactionPage `json:"allTransactions"`
}

type TransactionPage struct {
	TotalCount int           `json:"totalCount"`
	Results    []Transaction `json:"results"`
}

type Transaction struct {
	ID                 string      `json:"id"`
	Amount             float64     `json:"amount"`
	Date               string      `json:"date"`
	Pending            bool        `json:"pending"`
	Notes              string      `json:"notes"`
	IsRecurring        bool        `json:"isRecurring"`
	ReviewStatus       string      `json:"reviewStatus"`
	NeedsReview        bool        `json:"needsReview"`
	IsSplitTransaction bool        `json:"isSplitTransaction"`
	Category           Category    `json:"category"`
	Merchant           Merchant    `json:"merchant"`
	Account            TxAccount   `json:"account"`
}

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Merchant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TxAccount struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// Cashflow / Summary

type CashflowData struct {
	Summary TransactionSummary `json:"summary"`
}

type TransactionSummary struct {
	Avg        float64            `json:"avg"`
	Count      int                `json:"count"`
	Max        float64            `json:"max"`
	Sum        float64            `json:"sum"`
	ByCategory []CategorySummary  `json:"byCategory"`
}

type CategorySummary struct {
	GroupBy  CategoryGroupBy `json:"groupBy"`
	Summary  SummaryStats    `json:"summary"`
}

type CategoryGroupBy struct {
	Category CategoryDetail `json:"category"`
}

type CategoryDetail struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Group CategoryGroup `json:"group"`
}

type CategoryGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type SummaryStats struct {
	Sum   float64 `json:"sum"`
	Count int     `json:"count"`
	Avg   float64 `json:"avg"`
	Max   float64 `json:"max"`
}

// Budgets

type BudgetsData struct {
	Budget []BudgetItem `json:"budget"`
}

type BudgetItem struct {
	Category CategoryDetail `json:"category"`
	TotalBudgeted float64   `json:"totalBudgeted"`
	TotalActual   float64   `json:"totalActual"`
	TotalRemaining float64  `json:"totalRemaining"`
	MonthlyAmounts []MonthlyBudget `json:"monthlyAmounts"`
}

type MonthlyBudget struct {
	Month     string  `json:"month"`
	Budgeted  float64 `json:"budgeted"`
	Actual    float64 `json:"actual"`
	Remaining float64 `json:"remaining"`
}

// Net worth

type NetWorthData struct {
	NetWorthSummary NetWorthSummary `json:"netWorthSummary"`
}

type NetWorthSummary struct {
	NetWorth  float64          `json:"netWorth"`
	Breakdown []NetWorthItem   `json:"breakdown"`
}

type NetWorthItem struct {
	Type     string  `json:"type"`
	Balance  float64 `json:"balance"`
	Percent  float64 `json:"percent"`
}
