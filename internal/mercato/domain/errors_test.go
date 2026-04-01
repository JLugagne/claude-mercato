package domain

import (
	"errors"
	"fmt"
	"testing"
)

// TestDomainError_Unwrap_Nil verifies that Unwrap returns nil when Err is nil.
func TestDomainError_Unwrap_Nil(t *testing.T) {
	de := &DomainError{Code: "TEST", Message: "test error"}
	if got := de.Unwrap(); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// TestDomainError_Unwrap_Set verifies that Unwrap returns the wrapped error.
func TestDomainError_Unwrap_Set(t *testing.T) {
	inner := errors.New("inner error")
	de := &DomainError{Code: "TEST", Message: "test error", Err: inner}
	got := de.Unwrap()
	if got != inner {
		t.Errorf("expected %v, got %v", inner, got)
	}
}

// TestDomainError_Wrap verifies that Wrap creates a new DomainError preserving
// Code and Message while setting the wrapped error.
func TestDomainError_Wrap(t *testing.T) {
	original := &DomainError{Code: "ORIG_CODE", Message: "original message"}
	cause := errors.New("root cause")

	wrapped := original.Wrap(cause)

	if wrapped == original {
		t.Error("Wrap should return a new DomainError, not mutate the original")
	}
	if wrapped.Code != "ORIG_CODE" {
		t.Errorf("expected Code=ORIG_CODE, got %q", wrapped.Code)
	}
	if wrapped.Message != "original message" {
		t.Errorf("expected Message='original message', got %q", wrapped.Message)
	}
	if wrapped.Err != cause {
		t.Errorf("expected Err to be the cause, got %v", wrapped.Err)
	}
	// Verify the error chain works with errors.Is.
	if !errors.Is(wrapped, cause) {
		t.Error("errors.Is should find the cause in the chain")
	}
}

// TestDomainError_Error_WithoutWrapped verifies the format without a wrapped error.
func TestDomainError_Error_WithoutWrapped(t *testing.T) {
	de := &DomainError{Code: "MY_CODE", Message: "something failed"}
	want := "MY_CODE: something failed"
	if got := de.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestDomainError_Error_WithWrapped verifies the format with a wrapped error.
func TestDomainError_Error_WithWrapped(t *testing.T) {
	inner := errors.New("disk full")
	de := &DomainError{Code: "MY_CODE", Message: "write failed", Err: inner}
	want := fmt.Sprintf("MY_CODE: write failed: %v", inner)
	if got := de.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
