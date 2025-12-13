package govship_test

import (
	"testing"

	vship "github.com/GreatValueCreamSoda/govship"
)

func Test_ExceptionCode_IsNone(t *testing.T) {
	if !vship.ExceptionCodeNoError.IsNone() {
		t.Fatal("ExceptionCodeNoError should report IsNone() == true")
	}

	if vship.ExceptionCodeBadHandler.IsNone() {
		t.Fatal("non-zero ExceptionCode should report IsNone() == false")
	}
}
