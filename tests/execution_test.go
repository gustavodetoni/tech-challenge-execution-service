package tests

import (
	"testing"

	"github.com/gustavodetoni/tech-challenge-execution-service/internal/application"
)

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{{"QUEUED", "IN_PROGRESS", true}, {"QUEUED", "CANCELED", true}, {"IN_PROGRESS", "FINISHED", true}, {"IN_PROGRESS", "CANCELED", true}, {"FINISHED", "IN_PROGRESS", false}, {"", "FINISHED", false}}
	for _, tc := range cases {
		if got := application.CanTransition(tc.from, tc.to); got != tc.want {
			t.Errorf("%s -> %s: got %v want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestValidateExecutionOrder(t *testing.T) {
	if application.ValidateExecutionOrder("") == nil {
		t.Fatal("empty order must fail")
	}
	if application.ValidateExecutionOrder("order-1") != nil {
		t.Fatal("valid order must pass")
	}
}

func TestEventForStatus(t *testing.T) {
	cases := map[string]string{"IN_PROGRESS": "ExecutionStarted", "FINISHED": "ServiceOrderExecutionCompleted", "CANCELED": "ExecutionCanceled", "FAILED": "ExecutionFailed", "QUEUED": "ExecutionQueued"}
	for status, want := range cases {
		if got := application.EventForStatus(status); got != want {
			t.Errorf("%s: got %s want %s", status, got, want)
		}
	}
}
