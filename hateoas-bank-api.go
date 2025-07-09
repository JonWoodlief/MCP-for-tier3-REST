package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Link struct {
	Href   string `json:"href"`
	Method string `json:"method,omitempty"`
	Rel    string `json:"rel,omitempty"`
}

type Account struct {
	AccountID     string          `json:"accountId"`
	AccountHolder string          `json:"accountHolder"`
	Balance       float64         `json:"balance"`
	Currency      string          `json:"currency"`
	Links         map[string]Link `json:"_links"`
}

type Transaction struct {
	Amount float64 `json:"amount"`
}

var accounts = map[string]*Account{
	"acc-123": {
		AccountID:     "acc-123",
		AccountHolder: "John Doe",
		Balance:       1250.75,
		Currency:      "USD",
	},
	"acc-456": {
		AccountID:     "acc-456",
		AccountHolder: "Jane Smith",
		Balance:       -150.25,
		Currency:      "USD",
	},
}

func addHATEOASLinks(account *Account, baseURL string) {
	links := make(map[string]Link)

	// Self link - always present
	links["self"] = Link{
		Href:   fmt.Sprintf("%s/accounts/%s", baseURL, account.AccountID),
		Method: "GET",
		Rel:    "self",
	}

	// Deposit link - always available
	links["deposit"] = Link{
		Href:   fmt.Sprintf("%s/accounts/%s/deposit", baseURL, account.AccountID),
		Method: "POST",
		Rel:    "deposit",
	}

	// Withdraw link - only available if balance is positive
	if account.Balance > 0 {
		links["withdraw"] = Link{
			Href:   fmt.Sprintf("%s/accounts/%s/withdraw", baseURL, account.AccountID),
			Method: "POST",
			Rel:    "withdraw",
		}
	}

	account.Links = links
}

func getAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	accountID := strings.TrimPrefix(r.URL.Path, "/accounts/")

	account, exists := accounts[accountID]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Create a copy to avoid modifying the original
	accountCopy := *account
	baseURL := fmt.Sprintf("http://%s", r.Host)
	addHATEOASLinks(&accountCopy, baseURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accountCopy)
}

func deposit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	accountID := parts[2]

	account, exists := accounts[accountID]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var transaction Transaction
	if err := json.NewDecoder(r.Body).Decode(&transaction); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if transaction.Amount <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Amount must be positive"})
		return
	}

	account.Balance += transaction.Amount

	// Return updated account with HATEOAS links
	accountCopy := *account
	baseURL := fmt.Sprintf("http://%s", r.Host)
	addHATEOASLinks(&accountCopy, baseURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accountCopy)
}

func withdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	accountID := parts[2]

	account, exists := accounts[accountID]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Check if withdraw is allowed based on current balance
	if account.Balance <= 0 {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Withdrawal not allowed with negative balance"})
		return
	}

	var transaction Transaction
	if err := json.NewDecoder(r.Body).Decode(&transaction); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if transaction.Amount <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Amount must be positive"})
		return
	}

	account.Balance -= transaction.Amount

	// Return updated account with HATEOAS links
	accountCopy := *account
	baseURL := fmt.Sprintf("http://%s", r.Host)
	addHATEOASLinks(&accountCopy, baseURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accountCopy)
}

func main() {
	http.HandleFunc("/accounts/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasSuffix(path, "/deposit") {
			deposit(w, r)
		} else if strings.HasSuffix(path, "/withdraw") {
			withdraw(w, r)
		} else {
			getAccount(w, r)
		}
	})

	fmt.Println("HATEOAS Bank API server starting on :9001")
	fmt.Println("Try: curl http://localhost:9001/accounts/acc-123")
	fmt.Println("Try: curl http://localhost:9001/accounts/acc-456")
	fmt.Println("Try: curl -X POST -H 'Content-Type: application/json' -d '{\"amount\": 100}' http://localhost:9001/accounts/acc-456/deposit")

	err := http.ListenAndServe(":9001", http.DefaultServeMux)
	if err != nil {
		fmt.Println("Server failed to start:", err)
	}
}
