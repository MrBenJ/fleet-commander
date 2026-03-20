package queue

import (
	"fmt"
	"time"
)

// Request represents a user input request from an agent
type Request struct {
	ID        string    `json:"id"`
	AgentName string    `json:"agent_name"`
	Branch    string    `json:"branch"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // pending, active, completed
}

// Queue manages pending user input requests
type Queue struct {
	Requests []*Request
}

// New creates a new queue
func New() *Queue {
	return &Queue{
		Requests: []*Request{},
	}
}

// Add adds a new request to the queue
func (q *Queue) Add(agentName, branch, message string) *Request {
	req := &Request{
		ID:        fmt.Sprintf("%s-%d", agentName, time.Now().Unix()),
		AgentName: agentName,
		Branch:    branch,
		Message:   message,
		Timestamp: time.Now(),
		Status:    "pending",
	}
	
	q.Requests = append(q.Requests, req)
	return req
}

// GetPending returns all pending requests
func (q *Queue) GetPending() []*Request {
	var pending []*Request
	for _, r := range q.Requests {
		if r.Status == "pending" {
			pending = append(pending, r)
		}
	}
	return pending
}

// GetNext returns the next pending request
func (q *Queue) GetNext() *Request {
	for _, r := range q.Requests {
		if r.Status == "pending" {
			return r
		}
	}
	return nil
}

// Activate marks a request as active
func (q *Queue) Activate(id string) error {
	for _, r := range q.Requests {
		if r.ID == id {
			r.Status = "active"
			return nil
		}
	}
	return fmt.Errorf("request %s not found", id)
}

// Complete marks a request as completed
func (q *Queue) Complete(id string) error {
	for _, r := range q.Requests {
		if r.ID == id {
			r.Status = "completed"
			return nil
		}
	}
	return fmt.Errorf("request %s not found", id)
}

// Len returns the number of pending requests
func (q *Queue) Len() int {
	return len(q.GetPending())
}
