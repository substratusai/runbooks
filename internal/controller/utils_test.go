package controller

import "testing"

func Test_nextPowOf2(t *testing.T) {
	testCases := []struct {
		input    int64
		expected int64
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{6, 8},
		{7, 8},
		{8, 8},
	}
	for _, tc := range testCases {
		actual := nextPowOf2(tc.input)
		if actual != tc.expected {
			t.Errorf("nextPowOf(%d): expected %d, actual %d", tc.input, tc.expected, actual)
		}
	}
}
