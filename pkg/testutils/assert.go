package testutils

import (
	"reflect"
	"testing"
)

func AssertEqual(t *testing.T, a interface{}, b interface{}) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("%q != %q", a, b)
	}
}

func AssertNotEqual(t *testing.T, a interface{}, b interface{}) {
	t.Helper()
	if reflect.DeepEqual(a, b) {
		t.Fatalf("%q == %q", a, b)
	}
}

func AssertError(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error. found empty")
	}
	errMsg := err.Error()
	if errMsg != expected {
		t.Fatalf("error %q != %q", errMsg, expected)
	}
}

func AssertNonError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected empty error. found %q", err)
	}
}
