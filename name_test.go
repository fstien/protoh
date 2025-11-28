package main

import "testing"

func TestName(t *testing.T) {
	if IsASCIIAlphanumeric("LargeFrank760") == false {
		t.Error()
	}
}
