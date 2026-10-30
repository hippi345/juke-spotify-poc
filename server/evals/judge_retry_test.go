package evals

import (
	"context"
	"fmt"
	"testing"
)

func TestJudgeErrRetriable_timeoutAnd5xx(t *testing.T) {
	if !judgeErrRetriable(context.DeadlineExceeded) {
		t.Fatal("deadline exceeded should be retriable")
	}
	if !judgeErrRetriable(fmt.Errorf("Post \"https://api\": context deadline exceeded (Client.Timeout exceeded)")) {
		t.Fatal("client timeout message should be retriable")
	}
	if judgeErrRetriable(fmt.Errorf("judge gemini HTTP 400: bad request")) {
		t.Fatal("4xx should not be retriable")
	}
	if !judgeErrRetriable(fmt.Errorf("judge gemini HTTP 503: unavailable")) {
		t.Fatal("503 should be retriable")
	}
}
