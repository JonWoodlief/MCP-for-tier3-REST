package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

var balance = 1250.75

var account = &Account{
	AccountID:     "acc-123",
	AccountHolder: "John Doe",
	Balance:       balance,
	Currency:      "USD",
}

func addHATEOASLinks(account *Account, baseURL string) {
	links := make(map[string]Link)

	// Self link - always present
	links["self"] = Link{
		Href:   fmt.Sprintf("%s/account", baseURL),
		Method: "GET",
		Rel:    "self",
	}

	// Deposit link - always available
	links["deposit"] = Link{
		Href:   fmt.Sprintf("%s/account/deposit", baseURL),
		Method: "POST",
		Rel:    "deposit",
	}

	// Withdraw link - only available if balance is positive
	if account.Balance > 0 {
		links["withdraw"] = Link{
			Href:   fmt.Sprintf("%s/account/withdraw", baseURL),
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

	baseURL := fmt.Sprintf("http://%s", r.Host)
	addHATEOASLinks(account, baseURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(account)
}

func deposit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
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
	baseURL := fmt.Sprintf("http://%s", r.Host)
	addHATEOASLinks(account, baseURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(account)
}

func withdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Check if withdraw is allowed based on current balance
	if balance <= 0 {
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
	baseURL := fmt.Sprintf("http://%s", r.Host)
	addHATEOASLinks(account, baseURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(account)
}

func main() {
	fs := http.FileServer(http.Dir("bank-api/static"))
	http.Handle("/", fs)

	http.HandleFunc("/account", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
		if r.Method == "GET" {
			getAccount(w, r)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/account/deposit", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
		deposit(w, r)
	})
	http.HandleFunc("/account/withdraw", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
		withdraw(w, r)
	})

	fmt.Println("HATEOAS Bank API server starting on :9001")
	fmt.Println("Try: open http://localhost:9001/")
	fmt.Println("Try: curl http://localhost:9001/account")
	fmt.Println("Try: curl -X POST -H 'Content-Type: application/json' -d '{\"amount\": 100}' http://localhost:9001/account/deposit")

	err := http.ListenAndServe(":9001", http.DefaultServeMux)
	if err != nil {
		fmt.Println("Server failed to start:", err)
	}
}
