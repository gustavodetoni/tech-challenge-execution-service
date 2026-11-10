package application

import "fmt"

func CanTransition(from, to string) bool {
	switch from {
	case "QUEUED":
		return to == "IN_PROGRESS" || to == "CANCELED"
	case "IN_PROGRESS":
		return to == "FINISHED" || to == "CANCELED"
	default:
		return false
	}
}

func ValidateExecutionOrder(orderID string) error {
	if orderID == "" {
		return fmt.Errorf("order_id is required")
	}
	return nil
}

func EventForStatus(status string) string {
	switch status {
	case "IN_PROGRESS":
		return "ExecutionStarted"
	case "FINISHED":
		return "ServiceOrderExecutionCompleted"
	case "CANCELED":
		return "ExecutionCanceled"
	default:
		return "ExecutionQueued"
	}
}
