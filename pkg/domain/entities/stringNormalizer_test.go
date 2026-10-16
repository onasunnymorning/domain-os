package entities

import (
	"fmt"
	"reflect"
	"testing"
)

func TestRemoveNewlines(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello\nWorld",
			expected: "Hello World",
		},
		{
			input:    "This is a\n\nmultiline\nstring",
			expected: "This is a  multiline string",
		},
		{
			input:    "No newlines here",
			expected: "No newlines here",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			actual := RemoveNewlines(tc.input)
			if actual != tc.expected {
				t.Errorf("RemoveNewlines(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}
func TestRemoveTabs(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello\tWorld",
			expected: "Hello World",
		},
		{
			input:    "This\tis\ta\ttabbed\t\tstring",
			expected: "This is a tabbed  string",
		},
		{
			input:    "No tabs here",
			expected: "No tabs here",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			actual := RemoveTabs(tc.input)
			if actual != tc.expected {
				t.Errorf("RemoveTabs(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestRemoveCarriageReturns(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello\rWorld",
			expected: "Hello World",
		},
		{
			input:    "This is a\rmultiline\r\rstring",
			expected: "This is a multiline  string",
		},
		{
			input:    "No carriage returns here",
			expected: "No carriage returns here",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			actual := RemoveCarriageReturns(tc.input)
			if actual != tc.expected {
				t.Errorf("RemoveCarriageReturns(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}
func TestReplaceMultipleSpaces(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello  World",
			expected: "Hello World",
		},
		{
			input:    "This   is   a   string   with   multiple   spaces",
			expected: "This is a string with multiple spaces",
		},
		{
			input:    "No spaces here",
			expected: "No spaces here",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			actual := ReplaceMultipleSpaces(tc.input)
			if actual != tc.expected {
				t.Errorf("ReplaceMultipleSpaces(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestRemoveTrailingDot(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello World.",
			expected: "Hello World",
		},
		{
			input:    "This is a string.",
			expected: "This is a string",
		},
		{
			input:    "No trailing dot here",
			expected: "No trailing dot here",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			actual := RemoveTrailingDot(tc.input)
			if actual != tc.expected {
				t.Errorf("RemoveTrailingDot(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestStandardizeString(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello\nWorld",
			expected: "Hello World",
		},
		{
			input:    "This is a\n\nmultiline\nstring",
			expected: "This is a multiline string",
		},
		{
			input:    "No newlines here",
			expected: "No newlines here",
		},
		{
			input:    "Hello\tWorld",
			expected: "Hello World",
		},
		{
			input:    "This\tis\ta\ttabbed\t\tstring",
			expected: "This is a tabbed string",
		},
		{
			input:    "No tabs here",
			expected: "No tabs here",
		},
		{
			input:    "Hello\rWorld",
			expected: "Hello World",
		},
		{
			input:    "This is a\rmultiline\r\rstring",
			expected: "This is a multiline string",
		},
		{
			input:    "No carriage returns here",
			expected: "No carriage returns here",
		},
		{
			input:    "Hello  World",
			expected: "Hello World",
		},
		{
			input:    "This   is   a   string   with   multiple   spaces",
			expected: "This is a string with multiple spaces",
		},
		{
			input:    " No  spaces  here   ",
			expected: "No spaces here",
		},
		{
			input:    "Example Inc.",
			expected: "Example Inc.", // the dot is part of the name
		},
		{
			input:    "Hello World. ",
			expected: "Hello World.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			actual := NormalizeString(tc.input)
			if actual != tc.expected {
				t.Errorf("StandardizeString(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}
func TestNormalizeEmail(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"Arthurwychan@gmail.com.", "arthurwychan@gmail.com"}, // as in IANA's registrar list
		{" ops@Example.com ", "ops@example.com"},
		{"ops@example.com", "ops@example.com"},
		{"first.last@example.com", "first.last@example.com"}, // only a trailing dot goes
	} {
		if got := NormalizeEmail(tc.input); got != tc.want {
			t.Errorf("NormalizeEmail(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func TestStandardizeStringSlice(t *testing.T) {
	testCases := []struct {
		input    []string
		expected []string
	}{
		{
			input:    []string{"Hello\nWorld", "This is a\n\nmultiline\nstring", "No newlines here"},
			expected: []string{"Hello World", "This is a multiline string", "No newlines here"},
		},
		{
			input:    []string{"Hello\tWorld", "This\tis\ta\ttabbed\t\tstring", "No tabs here"},
			expected: []string{"Hello World", "This is a tabbed string", "No tabs here"},
		},
		{
			input:    []string{"Hello\rWorld", "This is a\rmultiline\r\rstring", "No carriage returns here"},
			expected: []string{"Hello World", "This is a multiline string", "No carriage returns here"},
		},
		{
			input:    []string{"Hello  World", "This   is   a   string   with   multiple   spaces", "No spaces here"},
			expected: []string{"Hello World", "This is a string with multiple spaces", "No spaces here"},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.input), func(t *testing.T) {
			actual := NormalizeStringSlice(tc.input)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("StandardizeStringSlice(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}
