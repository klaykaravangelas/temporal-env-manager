package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/klaykaravangelas/temporal-env-manager/workflows/router"

	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

var temporalClient client.Client

func main() {
	var err error
	temporalClient, err = client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}
	defer temporalClient.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /environments", listEnvironments)
	mux.HandleFunc("POST /environments", createEnvironment)
	mux.HandleFunc("GET /environments/{id}", getEnvironment)
	mux.HandleFunc("DELETE /environments/{id}", deleteEnvironment)
	mux.HandleFunc("POST /environments/{id}/extend", extendEnvironment)

	log.Println("API server running on http://127.0.0.1:8090")
	log.Fatalln(http.ListenAndServe("127.0.0.1:8090", corsMiddleware(mux)))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func listEnvironments(w http.ResponseWriter, r *http.Request) {
	query := `ExecutionStatus = "Running"`

	resp, err := temporalClient.WorkflowService().ListWorkflowExecutions(r.Context(), &workflowservice.ListWorkflowExecutionsRequest{
		Namespace: "default",
		Query:     query,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type envSummary struct {
		WorkflowID string `json:"workflowId"`
		Type       string `json:"type"`
		StartTime  string `json:"startTime"`
	}

	var envs []envSummary
	for _, execution := range resp.Executions {
		parts := strings.SplitN(execution.Execution.WorkflowId, "-", 2)
		envType := parts[0]
		if envType == "router" {
			continue
		}
		envs = append(envs, envSummary{
			WorkflowID: execution.Execution.WorkflowId,
			Type:       envType,
			StartTime:  execution.StartTime.AsTime().Format(time.RFC3339),
		})
	}

	if envs == nil {
		envs = []envSummary{}
	}

	json.NewEncoder(w).Encode(envs)
}

func createEnvironment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type    string            `json:"type"`
		TTLMins *int              `json:"ttlMinutes"`
		Vars    map[string]string `json:"vars"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Type == "" {
		http.Error(w, `invalid body, expected {"type": "ec2", "ttlMinutes": 10, "vars": {}}`, http.StatusBadRequest)
		return
	}

	var ttl *time.Duration
	if body.TTLMins != nil {
		d := time.Duration(*body.TTLMins) * time.Minute
		ttl = &d
	}

	req := router.EnvironmentRequest{
		Type: body.Type,
		TTL:  ttl,
		Vars: body.Vars,
	}

	routerID := fmt.Sprintf("router-%d", time.Now().UnixMilli())
	options := client.StartWorkflowOptions{
		ID:        routerID,
		TaskQueue: "environment-task-queue",
	}

	we, err := temporalClient.ExecuteWorkflow(context.Background(), options, router.RouterWorkflow, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var childID string
	if err := we.Get(context.Background(), &childID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"workflowId": childID,
	})
}

func getEnvironment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	resp, err := temporalClient.QueryWorkflow(context.Background(), id, "", "status")
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var status map[string]string
	if err := resp.Get(&status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}

func deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	err := temporalClient.SignalWorkflow(context.Background(), id, "", "teardown", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"workflowId": id,
		"status":     "teardown signal sent",
	})
}

func extendEnvironment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Minutes int `json:"minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `invalid body, expected {"minutes": N}`, http.StatusBadRequest)
		return
	}

	extension := time.Duration(body.Minutes) * time.Minute

	err := temporalClient.SignalWorkflow(context.Background(), id, "", "extend-ttl", extension)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"workflowId": id,
		"status":     fmt.Sprintf("extended by %d minutes", body.Minutes),
	})
}
