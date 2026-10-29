package http1

import (
	"maps"
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

	emptyArgs := make(map[string]string)

	echoA := echoFunc("a")
	echoAB := echoFunc("a/b")
	echoABp := echoFunc("a/b/")
	echoAC := echoFunc("a/c")
	echoADp := echoFunc("a/d/")
	echoID := echoFunc("a/{id}/")
	echoIDModel := echoFunc("a/{id}/{model}")

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
	err = tree.Insert("/a/{id}/", echoID)
	if err != nil {
		t.Errorf("Insert() error = %v, key %v", err, "a/{id}/")
	}
	err = tree.Insert("/a/{id}/{model}", echoIDModel)
	if err != nil {
		t.Errorf("Insert() error = %v, key %v", err, "a/{id}/{model}")
	}

	tests := []struct {
		name        string
		path        string
		wantErr     bool
		wantHandler testFunc
		wantArgs    map[string]string
	}{
		{"testExactSingleMatch", "/a", false, echoA, emptyArgs},
		{"testExactDoubleMach", "/a/b", false, echoAB, emptyArgs},
		{"testPrefixDoubleMach", "/a/b/", false, echoABp, emptyArgs},
		{"testPrefixDoesNotCollapse", "/a/c/d/", true, nil, emptyArgs},
		{"testExactDoesCollapse", "/a/d", false, echoADp, emptyArgs},
		{"testExtractsArgs", "/a/test", false, echoID, map[string]string{"id": "test"}},
		{"testExtractsArgsWithTrailing", "/a/test/", false, echoID, map[string]string{"id": "test"}},
		{"testExtractsArgsInMiddle", "/a/test/path/thing", false, echoID, map[string]string{"id": "test"}},
		{"testExtractTwoArgs", "/a/test/audi", false, echoIDModel, map[string]string{"id": "test", "model": "audi"}},
		{"testExtractTwoArgs", "/a/test/audi/", false, echoID, map[string]string{"id": "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, args, err := tree.Find(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Find() error = %v, wantErr %v", err, tt.wantErr)
			}
			//if !tt.wantErr && !reflect.DeepEqual(handler, tt.wantHandler) {
			if !tt.wantErr && (*handler)() != tt.wantHandler() {
				t.Errorf("Find() handler = %s, wantHandler %s", (*handler)(), tt.wantHandler())
			}
			if !tt.wantErr && !maps.Equal(args, tt.wantArgs) {
				t.Errorf("Find() args = %v, wantArgs %v", args, tt.wantArgs)
			}
		})
	}
}
