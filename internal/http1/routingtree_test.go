package http1

import (
	"testing"
)

func TestRoutingTree_Find(t *testing.T) {

	type testFunc func() string
	tree := NewRoutingTree[testFunc]()

	echoFunc := func(s string) testFunc {
		return func() string {
			return s
		}
	}

	echoA := echoFunc("a")
	echoAB := echoFunc("a/b")
	echoABp := echoFunc("a/b/")
	echoAC := echoFunc("a/c")
	echoADp := echoFunc("a/d/")

	err := tree.Insert("/a", echoA)
	if err != nil {
		t.Errorf("Insert() error = %v, key %v", err, "a")
	}
	err = tree.Insert("/a/b", echoAB)
	if err != nil {
		t.Errorf("Insert() error = %v, key %v", err, "a/b")
	}
	err = tree.Insert("/a/b/", echoABp)
	if err != nil {
		t.Errorf("Insert() error = %v, key %v", err, "a/b/")
	}
	err = tree.Insert("/a/c", echoAC)
	if err != nil {
		t.Errorf("Insert() error = %v, key %v", err, "a/c")
	}
	err = tree.Insert("/a/d/", echoADp)
	if err != nil {
		t.Errorf("Insert() error = %v, key %v", err, "a/d/")
	}

	tests := []struct {
		name        string
		path        string
		wantErr     bool
		wantHandler testFunc
	}{
		{"testExactSingleMatch", "/a", false, echoA},
		{"testExactDoubleMach", "/a/b", false, echoAB},
		{"testPrefixDoubleMach", "/a/b/", false, echoABp},
		{"testPrefixDoesNotCollapse", "/a/c/", true, nil},
		{"testExactDoesCollapse", "/a/d", false, echoADp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := tree.Find(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Find() error = %v, wantErr %v", err, tt.wantErr)
			}
			//if !tt.wantErr && !reflect.DeepEqual(handler, tt.wantHandler) {
			if !tt.wantErr && (*handler)() != tt.wantHandler() {
				t.Errorf("Find() handler = %s, wantHandler %s", (*handler)(), tt.wantHandler())
			}
		})
	}
}
