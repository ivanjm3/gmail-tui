package api

import (
	"errors"
	"testing"
)

func TestAppErrorHelpers(t *testing.T) {
	cause := errors.New("root cause")

	transient := NewTransientError("temporary failure", cause)
	if transient.Kind != ErrTransient || transient.Error() != "temporary failure" {
		t.Fatalf("unexpected transient error: %+v", transient)
	}
	if !errors.Is(transient, cause) || transient.Unwrap() != cause {
		t.Fatalf("expected transient error to unwrap cause")
	}

	permanent := NewPermanentError("permanent failure", cause)
	if permanent.Kind != ErrPermanent || permanent.Error() != "permanent failure" {
		t.Fatalf("unexpected permanent error: %+v", permanent)
	}
	if !errors.Is(permanent, cause) || permanent.Unwrap() != cause {
		t.Fatalf("expected permanent error to unwrap cause")
	}
}
