package logger

import (
	"testing"
)

func TestNewLogger(t *testing.T) {
	t.Run("Initialize development logger", func(t *testing.T) {
		log, err := New("development")
		if err != nil {
			t.Fatalf("failed to create development logger: %v", err)
		}
		if log == nil {
			t.Fatal("expected non-nil logger")
		}
	})

	t.Run("Initialize production logger", func(t *testing.T) {
		log, err := New("production")
		if err != nil {
			t.Fatalf("failed to create production logger: %v", err)
		}
		if log == nil {
			t.Fatal("expected non-nil logger")
		}
	})
}
