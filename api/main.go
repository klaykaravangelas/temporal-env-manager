package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	envworkflow "github.com/klaykaravangelas/temporal-env-manager/workflow"

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

	http.HandleFunc("POST /environments", createEnvironment)
	http.HandleFunc("GET /environments/{id}", getEnvironment)
	http.HandleFunc("DELETE /environments/{id}", deleteEnvironment)
	http.HandleFunc("POST /environments/{id}/extend", extendEnvironment)

	log.Println("API server running on http://127.0.0.1:8090")
	log.Fatalln(http.ListenAndServe("127.0.0.1:8090", nil))
}

func createEnvironment(w http.ResponseWriter, r *http.Request) {
	cfg := envworkflow.EnvironmentConfig{
		TTL: 5 * time.Minute,
	}

	options := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("env-%d", time.Now().UnixMilli()),
		TaskQueue: "environment-task-queue",
	}

	we, err := temporalClient.ExecuteWorkflow(context.Background(), options, envworkflow.EnvironmentWorkflow, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"workflowId": we.GetID(),
		"runId":      we.GetRunID(),
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
		http.Error(w, "invalid body, expected {\"minutes\": N}", http.StatusBadRequest)
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
