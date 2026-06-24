package main

import "testing"

func TestGetGreatestCommonDivisor(t *testing.T) {
	cases := []struct {
		width    int
		height   int
		expected int
	}{
		{
			width: 1920,
			height: 1080,
			expected: 120,
		},
		{
			width: 1080,
			height: 1920,
			expected: 120,
		},
		{
			width: 1024,
			height: 768,
			expected: 256,
		},
	}

	for _, c := range cases {
		actual := getGreatestCommonDivisor(c.width, c.height)
		if actual != c.expected {
			t.Errorf(`---------------------------------
Calculation Invalid.'
Expecting: %d
Actual:    %d
Fail
`, c.expected, actual)
		}
	}
}
