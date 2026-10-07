package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// Fictional account proofs arrive on stdin, never in logs, argv or manifests.
type proof struct {
	Email, Password, Household, Ingredient, PhotoSHA256, OldToken, StockKey string
	Stock                                                                   json.RawMessage
}

func probe(ctx context.Context) (any, error) {
	var input proof
	if err := readJSON(&input); err != nil || input.Email == "" || input.Password == "" {
		return nil, errors.New("invalid fictional recovery proof")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	call := func(method, route, token, key string, body []byte) (int, []byte, error) {
		r, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:8080"+route, bytes.NewReader(body))
		if err != nil {
			return 0, nil, errors.New("invalid probe request")
		}
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		response, err := client.Do(r)
		if err != nil {
			return 0, nil, errors.New("restored API unavailable")
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		return response.StatusCode, data, err
	}
	if status, _, err := call("GET", "/health/ready", "", "", nil); err != nil || status != 200 {
		return nil, errors.New("restored API is not ready")
	}
	if status, _, err := call("GET", "/api/me", input.OldToken, "", nil); err != nil || status != 401 {
		return nil, errors.New("previous deployment token was not rejected")
	}
	login, _ := json.Marshal(map[string]string{"account": input.Email, "password": input.Password})
	status, data, err := call("POST", "/api/login", "", "", login)
	var session struct{ Token string }
	if err != nil || status != 200 || json.Unmarshal(data, &session) != nil || session.Token == "" {
		return nil, errors.New("restored fictional account cannot log in")
	}
	root := "/api/households/" + input.Household
	status, data, err = call("GET", root+"/inventory", session.Token, "", nil)
	var inventory struct {
		Items []struct{ ID, Quantity string }
	}
	if err != nil || status != 200 || json.Unmarshal(data, &inventory) != nil || len(inventory.Items) != 1 || inventory.Items[0].ID != input.Ingredient || inventory.Items[0].Quantity != "300.000" {
		return nil, errors.New("restored fixture stock differs")
	}
	status, data, err = call("GET", root+"/ingredients/"+input.Ingredient+"/image", session.Token, "", nil)
	sum := sha256.Sum256(data)
	if err != nil || status != 200 || hex.EncodeToString(sum[:]) != input.PhotoSHA256 {
		return nil, errors.New("restored fixture photo differs")
	}
	if status, _, err = call("POST", root+"/stock", session.Token, input.StockKey, input.Stock); err != nil || status != 200 {
		return nil, errors.New("restored idempotency replay failed")
	}
	status, data, err = call("GET", root+"/inventory", session.Token, "", nil)
	if err != nil || status != 200 || json.Unmarshal(data, &inventory) != nil || len(inventory.Items) != 1 || inventory.Items[0].Quantity != "300.000" {
		return nil, errors.New("restored replay duplicated stock")
	}
	if status, _, err = call("GET", root+"/inventory", "", "", nil); err != nil || status != 401 {
		return nil, errors.New("restored anonymous access was accepted")
	}
	fresh, _ := json.Marshal(map[string]string{"ingredient_id": input.Ingredient, "quantity": "1", "reason": "manual", "source": "recovery-fixture"})
	if status, _, err = call("POST", root+"/stock", session.Token, input.StockKey+"-fresh", fresh); err != nil || status != 200 {
		return nil, errors.New("new write after restore failed")
	}
	status, data, err = call("GET", root+"/inventory", session.Token, "", nil)
	if err != nil || status != 200 || json.Unmarshal(data, &inventory) != nil || len(inventory.Items) != 1 || inventory.Items[0].Quantity != "301.000" {
		return nil, errors.New("fresh restored write differs")
	}
	return map[string]any{"passed": true, "checks": []string{"readiness", "old token rejection", "restored password login", "300g inventory", "photo SHA256", "pre-backup idempotency replay", "anonymous access rejected", "fresh stock write after restore"}}, nil
}
