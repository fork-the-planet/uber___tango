package storage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotFoundError_Error(t *testing.T) {
	err := &NotFoundError{Path: "test/path"}
	assert.NotEmpty(t, err.Error())
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "NotFoundError returns true",
			err:      &NotFoundError{Path: "test"},
			expected: true,
		},
		{
			name:     "Generic error returns false",
			err:      errors.New("generic error"),
			expected: false,
		},
		{
			name:     "Nil error returns false",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFound(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
