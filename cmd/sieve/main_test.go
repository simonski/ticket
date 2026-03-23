package main

import (
	"reflect"
	"testing"
)

func TestSieve(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected []int
	}{
		{
			name:     "n < 2",
			n:        1,
			expected: []int{},
		},
		{
			name:     "n = 2",
			n:        2,
			expected: []int{2},
		},
		{
			name:     "n = 10",
			n:        10,
			expected: []int{2, 3, 5, 7},
		},
		{
			name:     "n = 30",
			n:        30,
			expected: []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29},
		},
		{
			name:     "n = 0",
			n:        0,
			expected: []int{},
		},
		{
			name:     "n = 100",
			n:        100,
			expected: []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53, 59, 61, 67, 71, 73, 79, 83, 89, 97},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sieve(tt.n)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Sieve(%d) = %v, expected %v", tt.n, result, tt.expected)
			}
		})
	}
}
