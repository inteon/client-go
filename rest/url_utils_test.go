/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rest

import (
	"path"
	"testing"
)

func TestValidatesHostParameter(t *testing.T) {
	testCases := []struct {
		Host    string
		APIPath string

		URL string
		Err bool
	}{
		{"127.0.0.1", "", "http://127.0.0.1", false},
		{"127.0.0.1:8080", "", "http://127.0.0.1:8080", false},
		{"foo.bar.com", "", "http://foo.bar.com", false},
		{"http://host/prefix", "", "http://host/prefix", false},
		{"http://host", "", "http://host", false},
		{"http://host", "/", "http://host/", false},
		{"http://host", "/other", "http://host/other", false},
		{"host/server", "", "", true},
	}
	for i, testCase := range testCases {
		u, err := DefaultServerURL(testCase.Host, false)
		switch {
		case err == nil && testCase.Err:
			t.Errorf("expected error but was nil")
			continue
		case err != nil && !testCase.Err:
			t.Errorf("unexpected error %v", err)
			continue
		case err != nil:
			continue
		}
		u.Path = path.Join(u.Path, testCase.APIPath)
		if e, a := testCase.URL, u.String(); e != a {
			t.Errorf("%d: expected host %s, got %s", i, e, a)
			continue
		}
	}
}
