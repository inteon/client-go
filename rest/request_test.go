/*
Copyright 2014 The Kubernetes Authors.

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
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	http_client "github.com/go418/http-client"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/concurrent"
	"k8s.io/client-go/pkg/watch"
	restclientwatch "k8s.io/client-go/rest/watch"
	"k8s.io/client-go/tools/metrics"
	"k8s.io/client-go/util/flowcontrol"
	utiltesting "k8s.io/client-go/util/testing"
	"k8s.io/klog/v2"
	testingclock "k8s.io/utils/clock/testing"
)

func TestNewRequestSetsAccept(t *testing.T) {
	r := NewRequestWithClient(&url.URL{Path: "/path/"}, nil, nil).Verb("get")
	if r.headers.Get("Accept") != "" {
		t.Errorf("unexpected headers: %#v", r.headers)
	}
	r = NewRequestWithClient(&url.URL{Path: "/path/"}, newNegotiator(t), nil).Verb("get")
	if r.headers.Get("Accept") != "application/json" {
		t.Errorf("unexpected headers: %#v", r.headers)
	}
	r = NewRequestWithClient(&url.URL{Path: "/path/"}, newNegotiator(t), nil).Verb("get").GroupVersion(v1.SchemeGroupVersion).ExpectKind("Namespace")
	if r.headers.Get("Accept") != "application/vnd.kubernetes.protobuf, application/json" {
		t.Errorf("unexpected headers: %#v", r.headers)
	}
}

func clientForFunc(fn clientFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

type clientFunc func(req *http.Request) (*http.Response, error)

func (f clientFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRequestSetsHeaders(t *testing.T) {
	server := clientForFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected headers: %#v", req.Header)
		}
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
		}, nil
	})
	r := NewRequestWithClient(&url.URL{Path: "/path"}, newNegotiator(t), nil).Verb("get")
	r.c.Client = server

	// Check if all "issue" methods are setting headers.
	_ = r.Do(context.Background())
	_, _ = r.Watch(context.Background())
	_, _ = r.Stream(context.Background())
}

func TestRequestWithErrorWontChange(t *testing.T) {
	gvCopy := v1.SchemeGroupVersion
	original := Request{
		err: errors.New("test"),
		c:   &RESTClient{},
	}
	r := original
	changed := r.Param("foo", "bar").
		GroupVersion(gvCopy).
		AbsPath("/abs").
		Prefix("test").
		Suffix("testing").
		Namespace("new").
		Resource("foos").
		Name("bars").
		Body("foo").
		Timeout(time.Millisecond)
	if changed != &r {
		t.Errorf("returned request should point to the same object")
	}
	if !reflect.DeepEqual(changed, &original) {
		t.Errorf("expected %#v, got %#v", &original, changed)
	}
}

func TestRequestPreservesBaseTrailingSlash(t *testing.T) {
	r := &Request{c: &RESTClient{base: &url.URL{}}, pathPrefix: "/path/"}
	if s := r.URL().String(); s != "/path/" {
		t.Errorf("trailing slash should be preserved: %s", s)
	}
}

func TestRequestAbsPathPreservesTrailingSlash(t *testing.T) {
	r := (&Request{c: &RESTClient{base: &url.URL{}}}).AbsPath("/foo/")
	if s := r.URL().String(); s != "/foo/" {
		t.Errorf("trailing slash should be preserved: %s", s)
	}

	r = (&Request{c: &RESTClient{base: &url.URL{}}}).AbsPath("/foo/")
	if s := r.URL().String(); s != "/foo/" {
		t.Errorf("trailing slash should be preserved: %s", s)
	}
}

func TestRequestAbsPathJoins(t *testing.T) {
	r := (&Request{c: &RESTClient{base: &url.URL{}}}).AbsPath("foo/bar", "baz")
	if s := r.URL().String(); s != "foo/bar/baz" {
		t.Errorf("trailing slash should be preserved: %s", s)
	}
}

func TestRequestSetsNamespace(t *testing.T) {
	r := (&Request{
		c: &RESTClient{base: &url.URL{Path: "/"}},
	}).Namespace("foo")
	if r.namespace == "" {
		t.Errorf("namespace should be set: %#v", r)
	}

	if s := r.URL().String(); s != "namespaces/foo" {
		t.Errorf("namespace should be in path: %s", s)
	}
}

func TestRequestOrdersNamespaceInPath(t *testing.T) {
	r := (&Request{
		c:          &RESTClient{base: &url.URL{}},
		pathPrefix: "/test/",
	}).Name("bar").Resource("baz").Namespace("foo")
	if s := r.URL().String(); s != "/test/namespaces/foo/baz/bar" {
		t.Errorf("namespace should be in order in path: %s", s)
	}
}

func TestRequestOrdersSubResource(t *testing.T) {
	r := (&Request{
		c:          &RESTClient{base: &url.URL{}},
		pathPrefix: "/test/",
	}).Name("bar").Resource("baz").Namespace("foo").Suffix("test").SubResource("a", "b")
	if s := r.URL().String(); s != "/test/namespaces/foo/baz/bar/a/b/test" {
		t.Errorf("namespace should be in order in path: %s", s)
	}
}

func TestRequestSetTwiceError(t *testing.T) {
	if (&Request{}).Name("bar").Name("baz").err == nil {
		t.Errorf("setting name twice should result in error")
	}
	if (&Request{}).Namespace("bar").Namespace("baz").err == nil {
		t.Errorf("setting namespace twice should result in error")
	}
	if (&Request{}).Resource("bar").Resource("baz").err == nil {
		t.Errorf("setting resource twice should result in error")
	}
	if (&Request{}).SubResource("bar").SubResource("baz").err == nil {
		t.Errorf("setting subresource twice should result in error")
	}
}

func TestInvalidSegments(t *testing.T) {
	invalidSegments := []string{".", "..", "test/segment", "test%2bsegment"}
	setters := map[string]func(string, *Request){
		"namespace":   func(s string, r *Request) { r.Namespace(s) },
		"resource":    func(s string, r *Request) { r.Resource(s) },
		"name":        func(s string, r *Request) { r.Name(s) },
		"subresource": func(s string, r *Request) { r.SubResource(s) },
	}
	for _, invalidSegment := range invalidSegments {
		for setterName, setter := range setters {
			r := &Request{}
			setter(invalidSegment, r)
			if r.err == nil {
				t.Errorf("%s: %s: expected error, got none", setterName, invalidSegment)
			}
		}
	}
}

func TestRequestParam(t *testing.T) {
	r := (&Request{}).Param("foo", "a")
	if !reflect.DeepEqual(r.params, url.Values{"foo": []string{"a"}}) {
		t.Errorf("should have set a param: %#v", r)
	}

	r.Param("bar", "1")
	r.Param("bar", "2")
	if !reflect.DeepEqual(r.params, url.Values{"foo": []string{"a"}, "bar": []string{"1", "2"}}) {
		t.Errorf("should have set a param: %#v", r)
	}
}

func TestRequestVersionedParams(t *testing.T) {
	r := (&Request{c: &RESTClient{negotiator: newNegotiator(t)}}).GroupVersion(v1.SchemeGroupVersion).Param("foo", "a")
	if !reflect.DeepEqual(r.params, url.Values{"foo": []string{"a"}}) {
		t.Errorf("should have set a param: %#v", r)
	}
	r.VersionedParams(&v1.PodLogOptions{Follow: true, Container: "bar"})

	if !reflect.DeepEqual(r.params, url.Values{
		"foo":       []string{"a"},
		"container": []string{"bar"},
		"follow":    []string{"true"},
	}) {
		t.Errorf("should have set a param: %#v", r)
	}
}

func TestRequestVersionedParamsFromListOptions(t *testing.T) {
	r := (&Request{c: &RESTClient{negotiator: newNegotiator(t)}}).GroupVersion(v1.SchemeGroupVersion)
	r.VersionedParams(&metav1.ListOptions{ResourceVersion: "1"})
	if !reflect.DeepEqual(r.params, url.Values{
		"resourceVersion": []string{"1"},
	}) {
		t.Errorf("should have set a param: %#v", r)
	}

	var timeout int64 = 10
	r.VersionedParams(&metav1.ListOptions{ResourceVersion: "2", TimeoutSeconds: &timeout})
	if !reflect.DeepEqual(r.params, url.Values{
		"resourceVersion": []string{"1", "2"},
		"timeoutSeconds":  []string{"10"},
	}) {
		t.Errorf("should have set a param: %#v %v", r.params, r.err)
	}
}

func TestRequestURI(t *testing.T) {
	r := (&Request{}).Param("foo", "a")
	r.Prefix("other")
	r.RequestURI("/test?foo=b&a=b&c=1&c=2")
	if r.pathPrefix != "/test" {
		t.Errorf("path is wrong: %#v", r)
	}
	if !reflect.DeepEqual(r.params, url.Values{"a": []string{"b"}, "foo": []string{"b"}, "c": []string{"1", "2"}}) {
		t.Errorf("should have set a param: %#v", r)
	}
}

type NotAnAPIObject struct{}

func (obj NotAnAPIObject) GroupVersionKind() *schema.GroupVersionKind       { return nil }
func (obj NotAnAPIObject) SetGroupVersionKind(gvk *schema.GroupVersionKind) {}

func TestRequestBody(t *testing.T) {
	// test unknown type
	r := (&Request{}).Body([]string{"test"})
	if r.err == nil || r.body != nil {
		t.Errorf("should have set err and left body nil: %#v", r)
	}

	// test error set when failing to read file
	f, err := ioutil.TempFile("", "test")
	if err != nil {
		t.Fatalf("unable to create temp file")
	}
	defer f.Close()
	os.Remove(f.Name())
	r = (&Request{}).Body(f.Name())
	if r.err == nil || r.body != nil {
		t.Errorf("should have set err and left body nil: %#v", r)
	}

	// test unencodable api object
	r = (&Request{c: &RESTClient{negotiator: newNegotiator(t)}}).Body(&NotAnAPIObject{})
	if r.err == nil || r.body != nil {
		t.Errorf("should have set err and left body nil: %#v", r)
	}
}

func TestResultIntoWithErrReturnsErr(t *testing.T) {
	res := Result{err: errors.New("test")}
	if err := res.Into(&v1.Pod{}); err != res.err {
		t.Errorf("should have returned exact error from result")
	}
}

func TestResultIntoWithNoBodyReturnsErr(t *testing.T) {
	negotiator := newNegotiator(t)
	decoder, err := negotiator.Decoder(runtime.ContentTypeJSON, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := Result{
		body:    []byte{},
		decoder: decoder,
	}
	if err := res.Into(&v1.Pod{}); err == nil || !strings.Contains(err.Error(), "0-length") {
		t.Errorf("should have complained about 0 length body")
	}
}

func TestTransformResponse(t *testing.T) {
	invalid := []byte("aaaaa")
	uri, _ := url.Parse("http://localhost")
	testCases := []struct {
		Response *http.Response
		Data     []byte
		Created  bool
		Error    bool
		ErrFn    func(err error) bool
	}{
		{Response: &http.Response{StatusCode: http.StatusOK}, Data: []byte{}},
		{Response: &http.Response{StatusCode: http.StatusCreated}, Data: []byte{}, Created: true},
		{Response: &http.Response{StatusCode: 199}, Error: true},
		{Response: &http.Response{StatusCode: http.StatusInternalServerError}, Error: true},
		{Response: &http.Response{StatusCode: http.StatusUnprocessableEntity}, Error: true},
		{Response: &http.Response{StatusCode: http.StatusConflict}, Error: true},
		{Response: &http.Response{StatusCode: http.StatusNotFound}, Error: true},
		{Response: &http.Response{StatusCode: http.StatusUnauthorized}, Error: true},
		{
			Response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       ioutil.NopCloser(bytes.NewReader(invalid)),
			},
			Error: true,
			ErrFn: func(err error) bool {
				return err.Error() != "aaaaa" && apierrors.IsUnauthorized(err)
			},
		},
		{
			Response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Content-Type": []string{"text/any"}},
				Body:       ioutil.NopCloser(bytes.NewReader(invalid)),
			},
			Error: true,
			ErrFn: func(err error) bool {
				return strings.Contains(err.Error(), "server has asked for the client to provide") && apierrors.IsUnauthorized(err)
			},
		},
		{Response: &http.Response{StatusCode: http.StatusForbidden}, Error: true},
		{Response: &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewReader(invalid))}, Data: invalid},
		{Response: &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewReader(invalid))}, Data: invalid},
	}
	for i, test := range testCases {
		r := NewRequestWithClient(uri, newNegotiator(t), nil)
		if test.Response.Body == nil {
			test.Response.Body = ioutil.NopCloser(bytes.NewReader([]byte{}))
		}
		result := r.transformResponse(test.Response, &http.Request{})
		response, created, err := result.body, result.statusCode == http.StatusCreated, result.err
		hasErr := err != nil
		if hasErr != test.Error {
			t.Errorf("%d: unexpected error: %t %v", i, test.Error, err)
		} else if hasErr && test.Response.StatusCode > 399 {
			status, ok := err.(apierrors.APIStatus)
			if !ok {
				t.Errorf("%d: response should have been transformable into APIStatus: %v", i, err)
				continue
			}
			if int(status.Status().Code) != test.Response.StatusCode {
				t.Errorf("%d: status code did not match response: %#v", i, status.Status())
			}
		}
		if test.ErrFn != nil && !test.ErrFn(err) {
			t.Errorf("%d: error function did not match: %v", i, err)
		}
		if !(test.Data == nil && response == nil) && !apiequality.Semantic.DeepDerivative(test.Data, response) {
			t.Errorf("%d: unexpected response: %#v %#v", i, test.Data, response)
		}
		if test.Created != created {
			t.Errorf("%d: expected created %t, got %t", i, test.Created, created)
		}
	}
}

type renegotiator struct {
	called      bool
	contentType string
	params      map[string]string
	decoder     runtime.Decoder
	err         error
}

func (r *renegotiator) Decoder(contentType string, params map[string]string) (runtime.Decoder, error) {
	r.called = true
	r.contentType = contentType
	r.params = params
	return r.decoder, r.err
}

func (r *renegotiator) Encoder(contentType string, params map[string]string) (runtime.Encoder, error) {
	return nil, fmt.Errorf("UNIMPLEMENTED")
}

func (r *renegotiator) StreamDecoder(contentType string, params map[string]string) (runtime.Decoder, runtime.Serializer, runtime.Framer, error) {
	return nil, nil, nil, fmt.Errorf("UNIMPLEMENTED")
}

func TestTransformResponseNegotiate(t *testing.T) {
	invalid := []byte("aaaaa")
	uri, _ := url.Parse("http://localhost")
	decoder := jsonDecoder(t)
	testCases := []struct {
		Response *http.Response
		Data     []byte
		Created  bool
		Error    bool
		ErrFn    func(err error) bool

		ContentType       string
		Called            bool
		ExpectContentType string
		Decoder           runtime.Decoder
		NegotiateErr      error
	}{
		{
			ContentType: "application/json",
			Response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       ioutil.NopCloser(bytes.NewReader(invalid)),
			},
			Called:            true,
			ExpectContentType: "application/json",
			Error:             true,
			ErrFn: func(err error) bool {
				return err.Error() != "aaaaa" && apierrors.IsUnauthorized(err)
			},
		},
		{
			ContentType: "application/json",
			Response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Content-Type": []string{"application/protobuf"}},
				Body:       ioutil.NopCloser(bytes.NewReader(invalid)),
			},
			Decoder: decoder,

			Called:            true,
			ExpectContentType: "application/protobuf",

			Error: true,
			ErrFn: func(err error) bool {
				return err.Error() != "aaaaa" && apierrors.IsUnauthorized(err)
			},
		},
		{
			ContentType: "application/json",
			Response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{"Content-Type": []string{"application/,others"}},
			},
			Decoder: decoder,

			Error: true,
			ErrFn: func(err error) bool {
				return err.Error() == "Internal error occurred: mime: expected token after slash" && err.(apierrors.APIStatus).Status().Code == 500
			},
		},
		{
			// negotiate when no content type specified
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/any"}},
				Body:       ioutil.NopCloser(bytes.NewReader(invalid)),
			},
			Decoder:           decoder,
			Called:            true,
			ExpectContentType: "text/any",
		},
		{
			// negotiate when no response content type specified
			ContentType: "text/any",
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader(invalid)),
			},
			Decoder:           decoder,
			Called:            true,
			ExpectContentType: "text/any",
		},
		{
			// unrecognized content type is not handled
			ContentType: "application/json",
			Response: &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{"Content-Type": []string{"application/unrecognized"}},
				Body:       ioutil.NopCloser(bytes.NewReader(invalid)),
			},
			Decoder: decoder,

			NegotiateErr:      fmt.Errorf("aaaa"),
			Called:            true,
			ExpectContentType: "application/unrecognized",

			Error: true,
			ErrFn: func(err error) bool {
				return err.Error() != "aaaaa" && apierrors.IsNotFound(err)
			},
		},
	}
	for i, test := range testCases {
		r := NewRequestWithClient(uri, newNegotiator(t), nil)
		if test.Response.Body == nil {
			test.Response.Body = ioutil.NopCloser(bytes.NewReader([]byte{}))
		}
		result := r.transformResponse(test.Response, &http.Request{})
		_, err := result.body, result.err
		hasErr := err != nil
		if hasErr != test.Error {
			t.Errorf("%d: unexpected error: %t %v", i, test.Error, err)
			continue
		} else if hasErr && test.Response.StatusCode > 399 {
			status, ok := err.(apierrors.APIStatus)
			if !ok {
				t.Errorf("%d: response should have been transformable into APIStatus: %v", i, err)
				continue
			}
			if int(status.Status().Code) != test.Response.StatusCode {
				t.Errorf("%d: status code did not match response: %#v", i, status.Status())
			}
		}
		if test.ErrFn != nil && !test.ErrFn(err) {
			t.Errorf("%d: error function did not match: %v", i, err)
		}
		// if negotiator.called != test.Called {
		// 	t.Errorf("%d: negotiator called %t != %t", i, negotiator.called, test.Called)
		// }
		if !test.Called {
			continue
		}
		// if negotiator.contentType != test.ExpectContentType {
		// 	t.Errorf("%d: unexpected content type: %s", i, negotiator.contentType)
		// }
	}
}

func TestTransformUnstructuredError(t *testing.T) {
	testCases := []struct {
		Req *http.Request
		Res *http.Response

		Resource string
		Name     string

		ErrFn       func(error) bool
		Transformed error
	}{
		{
			Resource: "foo",
			Name:     "bar",
			Req: &http.Request{
				Method: "POST",
			},
			Res: &http.Response{
				StatusCode: http.StatusConflict,
				Body:       ioutil.NopCloser(bytes.NewReader(nil)),
			},
			ErrFn: apierrors.IsAlreadyExists,
		},
		{
			Resource: "foo",
			Name:     "bar",
			Req: &http.Request{
				Method: "PUT",
			},
			Res: &http.Response{
				StatusCode: http.StatusConflict,
				Body:       ioutil.NopCloser(bytes.NewReader(nil)),
			},
			ErrFn: apierrors.IsConflict,
		},
		{
			Resource: "foo",
			Name:     "bar",
			Req:      &http.Request{},
			Res: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       ioutil.NopCloser(bytes.NewReader(nil)),
			},
			ErrFn: apierrors.IsNotFound,
		},
		{
			Req: &http.Request{},
			Res: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       ioutil.NopCloser(bytes.NewReader(nil)),
			},
			ErrFn: apierrors.IsBadRequest,
		},
		{
			// status in response overrides transformed result
			Req: &http.Request{},
			Res: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","code":404}`))),
			},
			ErrFn: apierrors.IsBadRequest,
			Transformed: &apierrors.StatusError{
				ErrStatus: metav1.Status{Status: metav1.StatusFailure, Code: http.StatusNotFound},
			},
		},
		{
			// successful status is ignored
			Req: &http.Request{},
			Res: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"kind":"Status","apiVersion":"v1","status":"Success","code":404}`))),
			},
			ErrFn: apierrors.IsBadRequest,
		},
		{
			// empty object does not change result
			Req: &http.Request{},
			Res: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
			ErrFn: apierrors.IsBadRequest,
		},
		{
			// we default apiVersion for backwards compatibility with old clients
			// TODO: potentially remove in 1.7
			Req: &http.Request{},
			Res: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"kind":"Status","status":"Failure","code":404}`))),
			},
			ErrFn: apierrors.IsBadRequest,
			Transformed: &apierrors.StatusError{
				ErrStatus: metav1.Status{Status: metav1.StatusFailure, Code: http.StatusNotFound},
			},
		},
		{
			// we do not default kind
			Req: &http.Request{},
			Res: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"status":"Failure","code":404}`))),
			},
			ErrFn: apierrors.IsBadRequest,
		},
	}

	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			r := (&Request{
				c: &RESTClient{
					negotiator: newNegotiator(t),
				},
				resourceName: testCase.Name,
				resource:     testCase.Resource,
			})
			result := r.transformResponse(testCase.Res, testCase.Req)
			err := result.err
			if !testCase.ErrFn(err) {
				t.Fatalf("unexpected error: %v", err)
			}
			if !apierrors.IsUnexpectedServerError(err) {
				t.Errorf("unexpected error type: %v", err)
			}
			if len(testCase.Name) != 0 && !strings.Contains(err.Error(), testCase.Name) {
				t.Errorf("unexpected error string: %s", err)
			}
			if len(testCase.Resource) != 0 && !strings.Contains(err.Error(), testCase.Resource) {
				t.Errorf("unexpected error string: %s", err)
			}

			// verify Error() properly transforms the error
			transformed := result.Error()
			expect := testCase.Transformed
			if expect == nil {
				expect = err
			}
			if expect.Error() != transformed.Error() {
				t.Errorf("unexpected Error(): %s", diff.ObjectReflectDiff(expect, transformed))
			}

			// verify result.Get properly transforms the error
			if _, err := result.Get(); expect.Error() != err.Error() {
				t.Errorf("unexpected error on Get(): %s", diff.ObjectReflectDiff(expect, err))
			}

			// verify result.Into properly handles the error
			if err := result.Into(&v1.Pod{}); expect.Error() != err.Error() {
				t.Errorf("unexpected error on Into(): %s", diff.ObjectReflectDiff(expect, err))
			}

			// verify result.Raw leaves the error in the untransformed state
			if _, err := result.Raw(); result.err.Error() != err.Error() {
				t.Errorf("unexpected error on Raw(): %s", diff.ObjectReflectDiff(expect, err))
			}
		})
	}
}

func TestRequestWatch(t *testing.T) {
	testCases := []struct {
		name             string
		Request          *Request
		maxRetries       int
		serverReturns    []responseErr
		Expect           []watch.Event
		attemptsExpected int
		Err              bool
		ErrFn            func(error) bool
		Empty            bool
	}{
		{
			name:             "Request has error",
			Request:          &Request{err: errors.New("bail")},
			attemptsExpected: 0,
			Err:              true,
		},
		{
			name:    "Client is nil, should use http.DefaultClient",
			Request: &Request{c: &RESTClient{base: &url.URL{}}, pathPrefix: "%"},
			Err:     true,
		},
		{
			name: "error is not retryable",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: nil, err: errors.New("err")},
			},
			attemptsExpected: 1,
			Err:              true,
		},
		{
			name: "server returns forbidden",
			Request: &Request{
				c: &RESTClient{
					negotiator: newNegotiator(t),
					base:       &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: &http.Response{
					StatusCode: http.StatusForbidden,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
				}, err: nil},
			},
			attemptsExpected: 1,
			Expect: []watch.Event{
				{
					Type: watch.Error,
					Object: &metav1.Status{
						Status:  "Failure",
						Code:    500,
						Reason:  "InternalError",
						Message: `an error on the server ("unable to decode an event from the watch stream: test error") has prevented the request from succeeding`,
						Details: &metav1.StatusDetails{
							Causes: []metav1.StatusCause{
								{
									Type:    "UnexpectedServerResponse",
									Message: "unable to decode an event from the watch stream: test error",
								},
								{
									Type:    "ClientWatchDecoding",
									Message: "unable to decode an event from the watch stream: test error",
								},
							},
						},
					},
				},
			},
			Err: true,
			ErrFn: func(err error) bool {
				return apierrors.IsForbidden(err)
			},
		},
		{
			name: "server returns forbidden",
			Request: &Request{
				c: &RESTClient{
					negotiator: newNegotiator(t),
					base:       &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: &http.Response{
					StatusCode: http.StatusForbidden,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
				}, err: nil},
			},
			attemptsExpected: 1,
			Err:              true,
			ErrFn: func(err error) bool {
				return apierrors.IsForbidden(err)
			},
		},
		{
			name: "server returns unauthorized",
			Request: &Request{
				c: &RESTClient{
					negotiator: newNegotiator(t),
					base:       &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
				}, err: nil},
			},
			attemptsExpected: 1,
			Err:              true,
			ErrFn: func(err error) bool {
				return apierrors.IsUnauthorized(err)
			},
		},
		{
			name: "server returns unauthorized",
			Request: &Request{
				c: &RESTClient{
					negotiator: newNegotiator(t),
					base:       &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body: ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(jsonEncoder(t), &metav1.Status{
						Status: metav1.StatusFailure,
						Reason: metav1.StatusReasonUnauthorized,
					})))),
				}, err: nil},
			},
			attemptsExpected: 1,
			Err:              true,
			ErrFn: func(err error) bool {
				return apierrors.IsUnauthorized(err)
			},
		},
		{
			name: "server returns EOF error",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: nil, err: io.EOF},
			},
			attemptsExpected: 1,
			Empty:            true,
		},
		{
			name: "server returns can't write HTTP request on broken connection error",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: nil, err: errors.New("http: can't write HTTP request on broken connection")},
			},
			attemptsExpected: 1,
			Empty:            true,
		},
		{
			name: "server returns connection reset by peer",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: nil, err: errors.New("foo: connection reset by peer")},
			},
			attemptsExpected: 1,
			Empty:            true,
		},
		{
			name: "max retries 2, server always returns EOF error",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			maxRetries:       2,
			attemptsExpected: 3,
			serverReturns: []responseErr{
				{response: nil, err: io.EOF},
				{response: nil, err: io.EOF},
				{response: nil, err: io.EOF},
			},
			Empty: true,
		},
		{
			name: "max retries 1, server returns a retry-after response, request body seek error",
			Request: &Request{
				body: &readSeeker{err: io.EOF},
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			maxRetries:       1,
			attemptsExpected: 1,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
			},
			Err: true,
			ErrFn: func(err error) bool {
				return !apierrors.IsInternalError(err) && strings.Contains(err.Error(), "failed to reset the request body while retrying a request: EOF")
			},
		},
		{
			name: "max retries 1, server returns a retryable error, request body seek error",
			Request: &Request{
				body: &readSeeker{err: io.EOF},
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			maxRetries:       1,
			attemptsExpected: 1,
			serverReturns: []responseErr{
				{response: nil, err: io.EOF},
			},
			Err: true,
			ErrFn: func(err error) bool {
				return !apierrors.IsInternalError(err)
			},
		},
		{
			name: "max retries 2, server always returns a response with Retry-After header",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			maxRetries:       2,
			attemptsExpected: 3,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
			},
			Err: true,
			ErrFn: func(err error) bool {
				return apierrors.IsInternalError(err)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var attemptsGot int
			client := clientForFunc(func(req *http.Request) (*http.Response, error) {
				defer func() {
					attemptsGot++
				}()

				if attemptsGot >= len(testCase.serverReturns) {
					t.Fatalf("Wrong test setup, the server does not know what to return")
				}
				re := testCase.serverReturns[attemptsGot]
				return re.response, re.err
			})
			if c := testCase.Request.c; c != nil && len(testCase.serverReturns) > 0 {
				c.Client = client
			}
			testCase.Request.backoff = &noSleepBackOff{}
			testCase.Request.maxRetries = testCase.maxRetries
			testCase.Request.retryFn = defaultRequestRetryFn

			watcher, err := testCase.Request.Watch(ctx)

			if watcher == nil && err == nil {
				t.Fatal("Both watch.Watcher and err returned by Watch are nil")
			}
			if testCase.attemptsExpected != attemptsGot {
				t.Errorf("Expected RoundTrip to be invoked %d times, but got: %d", testCase.attemptsExpected, attemptsGot)
			}
			hasErr := err != nil
			if hasErr != testCase.Err {
				t.Fatalf("expected %t, got %t: %v", testCase.Err, hasErr, err)
			}
			if testCase.ErrFn != nil && !testCase.ErrFn(err) {
				t.Errorf("error not valid: %v", err)
			}
			if hasErr && watcher != nil {
				t.Fatalf("watch should be nil when error is returned")
			}
			if hasErr {
				return
			}

			outStream, ctx, cancelListen := concurrent.Watch(ctx, watcher)
			defer cancelListen()

			if testCase.Empty {
				evt, ok := <-outStream
				if ok {
					t.Errorf("expected the watch to be empty: %#v", evt)
				}
			}
			if testCase.Expect != nil {
				for i, evt := range testCase.Expect {
					out, ok := <-outStream
					if !ok {
						t.Fatalf("Watch closed early, %d/%d read", i, len(testCase.Expect))
					}
					if !reflect.DeepEqual(evt, out) {
						t.Fatalf("Event %d does not match: %s", i, diff.ObjectReflectDiff(evt, out))
					}
				}
			}
		})
	}
}

func TestRequestStream(t *testing.T) {
	testCases := []struct {
		name             string
		Request          *Request
		maxRetries       int
		serverReturns    []responseErr
		attemptsExpected int
		Err              bool
		ErrFn            func(error) bool
	}{
		{
			name:             "request has error",
			Request:          &Request{err: errors.New("bail")},
			attemptsExpected: 0,
			Err:              true,
		},
		{
			name:    "Client is nil, should use http.DefaultClient",
			Request: &Request{c: &RESTClient{base: &url.URL{}}, pathPrefix: "%"},
			Err:     true,
		},
		{
			name: "server returns an error",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: nil, err: errors.New("err")},
			},
			attemptsExpected: 1,
			Err:              true,
		},
		{
			name: "server returns an unauthorized error (encoding in response)",
			Request: &Request{
				c: &RESTClient{
					negotiator: newNegotiator(t),
					base:       &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body: ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(jsonEncoder(t), &metav1.Status{
						Status: metav1.StatusFailure,
						Reason: metav1.StatusReasonUnauthorized,
					})))),
				}, err: nil},
			},
			attemptsExpected: 1,
			Err:              true,
		},
		{
			name: "server returns a bad request error (encoding in request)",
			Request: (&Request{
				c: &RESTClient{
					negotiator: newNegotiator(t),
					base:       &url.URL{},
				},
			}).SetHeader("Accept", "application/json"),
			serverReturns: []responseErr{
				{response: &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"a container name must be specified for pod kube-dns-v20-mz5cv, choose one of: [kubedns dnsmasq healthz]","reason":"BadRequest","code":400}`))),
				}, err: nil},
			},
			attemptsExpected: 1,
			Err:              true,
			ErrFn: func(err error) bool {
				return err.Error() == "a container name must be specified for pod kube-dns-v20-mz5cv, choose one of: [kubedns dnsmasq healthz]"
			},
		},
		{
			name: "server returns a bad request error",
			Request: &Request{
				c: &RESTClient{
					negotiator: newNegotiator(t),
					base:       &url.URL{},
				},
			},
			serverReturns: []responseErr{
				{response: &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"a container name must be specified for pod kube-dns-v20-mz5cv, choose one of: [kubedns dnsmasq healthz]","reason":"BadRequest","code":400}`))),
				}, err: nil},
			},
			attemptsExpected: 1,
			Err:              true,
			ErrFn: func(err error) bool {
				return err.Error() == "a container name must be specified for pod kube-dns-v20-mz5cv, choose one of: [kubedns dnsmasq healthz]"
			},
		},
		{
			name: "max retries 1, server returns a retry-after response, request body seek error",
			Request: &Request{
				body: &readSeeker{err: io.EOF},
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			maxRetries:       1,
			attemptsExpected: 1,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
			},
			Err: true,
			ErrFn: func(err error) bool {
				return !apierrors.IsInternalError(err) && strings.Contains(err.Error(), "failed to reset the request body while retrying a request: EOF")
			},
		},
		{
			name: "max retries 2, server always returns a response with Retry-After header",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			maxRetries:       2,
			attemptsExpected: 3,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
			},
			Err: true,
			ErrFn: func(err error) bool {
				return apierrors.IsInternalError(err)
			},
		},
		{
			name: "server returns EOF after attempt 1, retry aborted",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			maxRetries:       2,
			attemptsExpected: 2,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: nil, err: io.EOF},
			},
			Err: true,
			ErrFn: func(err error) bool {
				return unWrap(err) == io.EOF
			},
		},
		{
			name: "max retries 2, server returns success on the final attempt",
			Request: &Request{
				c: &RESTClient{
					base: &url.URL{},
				},
			},
			maxRetries:       2,
			attemptsExpected: 3,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
				}, err: nil},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var attemptsGot int
			client := clientForFunc(func(req *http.Request) (*http.Response, error) {
				defer func() {
					attemptsGot++
				}()

				if attemptsGot >= len(testCase.serverReturns) {
					t.Fatalf("Wrong test setup, the server does not know what to return")
				}
				re := testCase.serverReturns[attemptsGot]
				return re.response, re.err
			})
			if c := testCase.Request.c; c != nil && len(testCase.serverReturns) > 0 {
				c.Client = client
			}
			testCase.Request.backoff = &noSleepBackOff{}
			testCase.Request.maxRetries = testCase.maxRetries
			testCase.Request.retryFn = defaultRequestRetryFn

			body, err := testCase.Request.Stream(context.Background())

			if body == nil && err == nil {
				t.Fatal("Both body and err returned by Stream are nil")
			}
			if testCase.attemptsExpected != attemptsGot {
				t.Errorf("Expected RoundTrip to be invoked %d times, but got: %d", testCase.attemptsExpected, attemptsGot)
			}

			hasErr := err != nil
			if hasErr != testCase.Err {
				t.Errorf("expected %t, got %t: %v", testCase.Err, hasErr, err)
			}
			if hasErr && body != nil {
				t.Error("body should be nil when error is returned")
			}

			if hasErr {
				if testCase.ErrFn != nil && !testCase.ErrFn(err) {
					t.Errorf("unexpected error: %#v", err)
				}
			}
		})
	}
}

func TestRequestDo(t *testing.T) {
	testCases := []struct {
		Request *Request
		Err     bool
	}{
		{
			Request: &Request{c: &RESTClient{}, err: errors.New("bail")},
			Err:     true,
		},
		{
			Request: &Request{c: &RESTClient{base: &url.URL{}}, pathPrefix: "%"},
			Err:     true,
		},
		{
			Request: &Request{
				c: &RESTClient{
					Client: clientForFunc(func(req *http.Request) (*http.Response, error) {
						return nil, errors.New("err")
					}),
					base: &url.URL{},
				},
			},
			Err: true,
		},
	}
	for i, testCase := range testCases {
		testCase.Request.backoff = &NoBackoff{}
		testCase.Request.retryFn = defaultRequestRetryFn
		body, err := testCase.Request.Do(context.Background()).Raw()
		hasErr := err != nil
		if hasErr != testCase.Err {
			t.Errorf("%d: expected %t, got %t: %v", i, testCase.Err, hasErr, err)
		}
		if hasErr && body != nil {
			t.Errorf("%d: body should be nil when error is returned", i)
		}
	}
}

func TestDoRequestNewWay(t *testing.T) {
	reqBody := "request body"
	expectedObj := &v1.Service{Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{
		Protocol:   "TCP",
		Port:       12345,
		TargetPort: intstr.FromInt(12345),
	}}}}
	expectedBody, _ := runtime.Encode(jsonEncoder(t), expectedObj)
	fakeHandler := utiltesting.FakeHandler{
		StatusCode:   200,
		ResponseBody: string(expectedBody),
		T:            t,
	}
	testServer := httptest.NewServer(&fakeHandler)
	defer testServer.Close()
	c := testRESTClient(t, testServer)
	obj, err := c.Verb("POST").
		ApiPath("/api").
		GroupVersion(v1.SchemeGroupVersion).
		Prefix("foo", "bar").
		Suffix("baz").
		Timeout(time.Second).
		Body([]byte(reqBody)).
		Do(context.Background()).Get()
	if err != nil {
		t.Errorf("Unexpected error: %v %#v", err, err)
		return
	}
	if obj == nil {
		t.Error("nil obj")
	} else if !apiequality.Semantic.DeepDerivative(expectedObj, obj) {
		t.Errorf("Expected: %#v, got %#v", expectedObj, obj)
	}
	requestURL := defaultResourcePathWithPrefix("foo/bar", "", "", "baz")
	requestURL += "?timeout=1s"
	fakeHandler.ValidateRequest(t, requestURL, "POST", &reqBody)
}

// This test assumes that the client implementation backs off exponentially, for an individual request.
func TestBackoffLifecycle(t *testing.T) {
	count := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count++
		t.Logf("Attempt %d", count)
		if count == 5 || count == 9 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer testServer.Close()
	c := testRESTClient(t, testServer)

	// Test backoff recovery and increase.  This correlates to the constants
	// which are used in the server implementation returning StatusOK above.
	seconds := []int{0, 1, 2, 4, 8, 0, 1, 2, 4, 0}
	request := c.Verb("POST").Prefix("backofftest").Suffix("abc")
	clock := testingclock.FakeClock{}
	request.backoff = &URLBackoff{
		// Use a fake backoff here to avoid flakes and speed the test up.
		Backoff: flowcontrol.NewFakeBackOff(
			time.Duration(1)*time.Second,
			time.Duration(200)*time.Second,
			&clock,
		)}

	for _, sec := range seconds {
		thisBackoff := request.backoff.CalculateBackoff(request.URL())
		t.Logf("Current backoff %v", thisBackoff)
		if thisBackoff != time.Duration(sec)*time.Second {
			t.Errorf("Backoff is %v instead of %v", thisBackoff, sec)
		}
		now := clock.Now()
		request.DoRaw(context.Background())
		elapsed := clock.Since(now)
		if clock.Since(now) != thisBackoff {
			t.Errorf("CalculatedBackoff not honored by clock: Expected time of %v, but got %v ", thisBackoff, elapsed)
		}
	}
}

type testBackoffManager struct {
	sleeps []time.Duration
}

func (b *testBackoffManager) UpdateBackoff(actualUrl *url.URL, err error, responseCode int) {
}

func (b *testBackoffManager) CalculateBackoff(actualUrl *url.URL) time.Duration {
	return time.Duration(0)
}

func (b *testBackoffManager) Sleep(d time.Duration) {
	b.sleeps = append(b.sleeps, d)
}

func TestCheckRetryClosesBody(t *testing.T) {
	count := 0
	ch := make(chan struct{})
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count++
		t.Logf("attempt %d", count)
		if count >= 5 {
			w.WriteHeader(http.StatusOK)
			close(ch)
			return
		}
		w.Header().Set("Retry-After", "1")
		http.Error(w, "Too many requests, please try again later.", http.StatusTooManyRequests)
	}))
	defer testServer.Close()

	backoff := &testBackoffManager{}

	// testBackoffManager.CalculateBackoff always returns 0
	expectedSleeps := []time.Duration{0, time.Second, time.Second, time.Second, time.Second}

	c := testRESTClient(t, testServer)
	c.createBackoffMgr = func() BackoffManager { return backoff }
	_, err := c.Verb("POST").
		Prefix("foo", "bar").
		Suffix("baz").
		Timeout(time.Second).
		Body([]byte(strings.Repeat("abcd", 1000))).
		DoRaw(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v %#v", err, err)
	}
	<-ch
	if count != 5 {
		t.Errorf("unexpected retries: %d", count)
	}
	if !reflect.DeepEqual(backoff.sleeps, expectedSleeps) {
		t.Errorf("unexpected sleeps, expected: %v, got: %v", expectedSleeps, backoff.sleeps)
	}
}

func TestConnectionResetByPeerIsRetried(t *testing.T) {
	count := 0
	backoff := &testBackoffManager{}
	req := &Request{
		verb: "GET",
		c: &RESTClient{
			Client: clientForFunc(func(req *http.Request) (*http.Response, error) {
				count++
				if count >= 3 {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
					}, nil
				}
				return nil, &net.OpError{Err: syscall.ECONNRESET}
			}),
		},
		backoff:    backoff,
		maxRetries: 10,
		retryFn:    defaultRequestRetryFn,
	}
	// We expect two retries of "connection reset by peer" and the success.
	_, err := req.Do(context.Background()).Raw()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 attempts, got: %d", count)
	}
	// We have a sleep before each retry (including the initial one) thus 3 together.
	if len(backoff.sleeps) != 3 {
		t.Errorf("Expected 3 backoff.Sleep, got: %d", len(backoff.sleeps))
	}
}

func TestCheckRetryHandles429And5xx(t *testing.T) {
	count := 0
	ch := make(chan struct{})
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		data, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("unable to read request body: %v", err)
		}
		if !bytes.Equal(data, []byte(strings.Repeat("abcd", 1000))) {
			t.Fatalf("retry did not send a complete body: %s", data)
		}
		t.Logf("attempt %d", count)
		if count >= 4 {
			w.WriteHeader(http.StatusOK)
			close(ch)
			return
		}
		w.Header().Set("Retry-After", "0")
		w.WriteHeader([]int{http.StatusTooManyRequests, 500, 501, 504}[count])
		count++
	}))
	defer testServer.Close()

	c := testRESTClient(t, testServer)
	_, err := c.Verb("POST").
		Prefix("foo", "bar").
		Suffix("baz").
		Timeout(time.Second).
		Body([]byte(strings.Repeat("abcd", 1000))).
		DoRaw(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v %#v", err, err)
	}
	<-ch
	if count != 4 {
		t.Errorf("unexpected retries: %d", count)
	}
}

func BenchmarkCheckRetryClosesBody(b *testing.B) {
	count := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count++
		if count%3 == 0 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer testServer.Close()

	c := testRESTClient(b, testServer)

	requests := make([]*Request, 0, b.N)
	for i := 0; i < b.N; i++ {
		requests = append(requests, c.Verb("POST").
			Prefix("foo", "bar").
			Suffix("baz").
			Timeout(time.Second).
			Body([]byte(strings.Repeat("abcd", 1000))))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := requests[i].DoRaw(context.Background()); err != nil {
			b.Fatalf("Unexpected error (%d/%d): %v", i, b.N, err)
		}
	}
}

func TestDoRequestNewWayReader(t *testing.T) {
	reqObj := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	reqBodyExpected, _ := runtime.Encode(jsonEncoder(t), reqObj)
	expectedObj := &v1.Service{Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{
		Protocol:   "TCP",
		Port:       12345,
		TargetPort: intstr.FromInt(12345),
	}}}}
	expectedBody, _ := runtime.Encode(jsonEncoder(t), expectedObj)
	fakeHandler := utiltesting.FakeHandler{
		StatusCode:   200,
		ResponseBody: string(expectedBody),
		T:            t,
	}
	testServer := httptest.NewServer(&fakeHandler)
	defer testServer.Close()
	c := testRESTClient(t, testServer)
	obj, err := c.Verb("POST").
		ApiPath("/api").
		GroupVersion(v1.SchemeGroupVersion).
		Resource("bar").
		Name("baz").
		Prefix("foo").
		Timeout(time.Second).
		Body(bytes.NewBuffer(reqBodyExpected)).
		Do(context.Background()).Get()
	if err != nil {
		t.Errorf("Unexpected error: %v %#v", err, err)
		return
	}
	if obj == nil {
		t.Error("nil obj")
	} else if !apiequality.Semantic.DeepDerivative(expectedObj, obj) {
		t.Errorf("Expected: %#v, got %#v", expectedObj, obj)
	}
	tmpStr := string(reqBodyExpected)
	requestURL := defaultResourcePathWithPrefix("foo", "bar", "", "baz")
	requestURL += "?timeout=1s"
	fakeHandler.ValidateRequest(t, requestURL, "POST", &tmpStr)
}

func TestDoRequestNewWayObj(t *testing.T) {
	reqObj := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	reqBodyExpected, _ := runtime.Encode(jsonEncoder(t), reqObj)
	expectedObj := &v1.Service{Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{
		Protocol:   "TCP",
		Port:       12345,
		TargetPort: intstr.FromInt(12345),
	}}}}
	expectedBody, _ := runtime.Encode(jsonEncoder(t), expectedObj)
	fakeHandler := utiltesting.FakeHandler{
		StatusCode:   200,
		ResponseBody: string(expectedBody),
		T:            t,
	}
	testServer := httptest.NewServer(&fakeHandler)
	defer testServer.Close()
	c := testRESTClient(t, testServer)
	obj, err := c.Verb("POST").
		ApiPath("/api").
		GroupVersion(v1.SchemeGroupVersion).
		Suffix("baz").
		Name("bar").
		Resource("foo").
		Timeout(time.Second).
		Body(reqObj).
		Do(context.Background()).Get()
	if err != nil {
		t.Errorf("Unexpected error: %v %#v", err, err)
		return
	}
	if obj == nil {
		t.Error("nil obj")
	} else if !apiequality.Semantic.DeepDerivative(expectedObj, obj) {
		t.Errorf("Expected: %#v, got %#v", expectedObj, obj)
	}
	tmpStr := string(reqBodyExpected)
	requestURL := defaultResourcePathWithPrefix("", "foo", "", "bar/baz")
	requestURL += "?timeout=1s"
	fakeHandler.ValidateRequest(t, requestURL, "POST", &tmpStr)
}

func TestDoRequestNewWayFile(t *testing.T) {
	reqObj := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	reqBodyExpected, err := runtime.Encode(jsonEncoder(t), reqObj)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	file, err := ioutil.TempFile("", "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	defer file.Close()
	defer os.Remove(file.Name())

	_, err = file.Write(reqBodyExpected)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedObj := &v1.Service{Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{
		Protocol:   "TCP",
		Port:       12345,
		TargetPort: intstr.FromInt(12345),
	}}}}
	expectedBody, _ := runtime.Encode(jsonEncoder(t), expectedObj)
	fakeHandler := utiltesting.FakeHandler{
		StatusCode:   200,
		ResponseBody: string(expectedBody),
		T:            t,
	}
	testServer := httptest.NewServer(&fakeHandler)
	defer testServer.Close()
	c := testRESTClient(t, testServer)
	wasCreated := true
	obj, err := c.Verb("POST").
		ApiPath("/api").
		GroupVersion(v1.SchemeGroupVersion).
		Prefix("foo/bar", "baz").
		Timeout(time.Second).
		Body(file.Name()).
		Do(context.Background()).WasCreated(&wasCreated).Get()
	if err != nil {
		t.Errorf("Unexpected error: %v %#v", err, err)
		return
	}
	if obj == nil {
		t.Error("nil obj")
	} else if !apiequality.Semantic.DeepDerivative(expectedObj, obj) {
		t.Errorf("Expected: %#v, got %#v", expectedObj, obj)
	}
	if wasCreated {
		t.Errorf("expected object was created")
	}
	tmpStr := string(reqBodyExpected)
	requestURL := defaultResourcePathWithPrefix("foo/bar/baz", "", "", "")
	requestURL += "?timeout=1s"
	fakeHandler.ValidateRequest(t, requestURL, "POST", &tmpStr)
}

func TestWasCreated(t *testing.T) {
	reqObj := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	reqBodyExpected, err := runtime.Encode(jsonEncoder(t), reqObj)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedObj := &v1.Service{Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{
		Protocol:   "TCP",
		Port:       12345,
		TargetPort: intstr.FromInt(12345),
	}}}}
	expectedBody, _ := runtime.Encode(jsonEncoder(t), expectedObj)
	fakeHandler := utiltesting.FakeHandler{
		StatusCode:   201,
		ResponseBody: string(expectedBody),
		T:            t,
	}
	testServer := httptest.NewServer(&fakeHandler)
	defer testServer.Close()
	c := testRESTClient(t, testServer)
	wasCreated := false
	obj, err := c.Verb("PUT").
		ApiPath("/api").
		GroupVersion(v1.SchemeGroupVersion).
		Prefix("foo/bar", "baz").
		Timeout(time.Second).
		Body(reqBodyExpected).
		Do(context.Background()).WasCreated(&wasCreated).Get()
	if err != nil {
		t.Errorf("Unexpected error: %v %#v", err, err)
		return
	}
	if obj == nil {
		t.Error("nil obj")
	} else if !apiequality.Semantic.DeepDerivative(expectedObj, obj) {
		t.Errorf("Expected: %#v, got %#v", expectedObj, obj)
	}
	if !wasCreated {
		t.Errorf("Expected object was created")
	}

	tmpStr := string(reqBodyExpected)
	requestURL := defaultResourcePathWithPrefix("foo/bar/baz", "", "", "")
	requestURL += "?timeout=1s"
	fakeHandler.ValidateRequest(t, requestURL, "PUT", &tmpStr)
}

func TestVerbs(t *testing.T) {
	c := testRESTClient(t, nil)
	if r := c.Post(); r.verb != "POST" {
		t.Errorf("Post verb is wrong")
	}
	if r := c.Put(); r.verb != "PUT" {
		t.Errorf("Put verb is wrong")
	}
	if r := c.Get(); r.verb != "GET" {
		t.Errorf("Get verb is wrong")
	}
	if r := c.Delete(); r.verb != "DELETE" {
		t.Errorf("Delete verb is wrong")
	}
}

func TestAbsPath(t *testing.T) {
	for i, tc := range []struct {
		configPrefix   string
		resourcePrefix string
		absPath        string
		wantsAbsPath   string
	}{
		{"/", "", "", "/"},
		{"", "", "/", "/"},
		{"", "", "/api", "/api"},
		{"", "", "/api/", "/api/"},
		{"", "", "/apis", "/apis"},
		{"", "/foo", "/bar/foo", "/bar/foo"},
		{"", "/api/foo/123", "/bar/foo", "/bar/foo"},
		{"/p1", "", "", "/p1"},
		{"/p1", "", "/", "/p1/"},
		{"/p1", "", "/api", "/p1/api"},
		{"/p1", "", "/apis", "/p1/apis"},
		{"/p1", "/r1", "/apis", "/p1/apis"},
		{"/p1", "/api/r1", "/apis", "/p1/apis"},
		{"/p1/api/p2", "", "", "/p1/api/p2"},
		{"/p1/api/p2", "", "/", "/p1/api/p2/"},
		{"/p1/api/p2", "", "/api", "/p1/api/p2/api"},
		{"/p1/api/p2", "", "/api/", "/p1/api/p2/api/"},
		{"/p1/api/p2", "/r1", "/api/", "/p1/api/p2/api/"},
		{"/p1/api/p2", "/api/r1", "/api/", "/p1/api/p2/api/"},
	} {
		u, _ := url.Parse("http://localhost:123" + tc.configPrefix)
		r := NewRequestWithClient(u, newNegotiator(t), nil).Verb("POST").Prefix(tc.resourcePrefix).AbsPath(tc.absPath)
		if r.pathPrefix != tc.wantsAbsPath {
			t.Errorf("test case %d failed, unexpected path: %q, expected %q", i, r.pathPrefix, tc.wantsAbsPath)
		}
	}
}

func TestUnacceptableParamNames(t *testing.T) {
	table := []struct {
		name          string
		testVal       string
		expectSuccess bool
	}{
		// timeout is no longer "protected"
		{"timeout", "42", true},
	}

	for _, item := range table {
		c := testRESTClient(t, nil)
		r := c.Get().setParam(item.name, item.testVal)
		if e, a := item.expectSuccess, r.err == nil; e != a {
			t.Errorf("expected %v, got %v (%v)", e, a, r.err)
		}
	}
}

func TestBody(t *testing.T) {
	const data = "test payload"

	obj := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	bodyExpected, _ := runtime.Encode(jsonEncoder(t), obj)

	f, err := ioutil.TempFile("", "test_body")
	if err != nil {
		t.Fatalf("TempFile error: %v", err)
	}
	if _, err := f.WriteString(data); err != nil {
		t.Fatalf("TempFile.WriteString error: %v", err)
	}
	f.Close()
	defer os.Remove(f.Name())

	var nilObject *metav1.DeleteOptions
	typedObject := interface{}(nilObject)
	c := testRESTClient(t, nil)
	tests := []struct {
		input    interface{}
		expected string
		headers  map[string]string
	}{
		{[]byte(data), data, nil},
		{f.Name(), data, nil},
		{strings.NewReader(data), data, nil},
		{obj, string(bodyExpected), map[string]string{"Content-Type": "application/json"}},
		{typedObject, "", nil},
	}
	for i, tt := range tests {
		r := c.Post().Body(tt.input)
		if r.err != nil {
			t.Errorf("%d: r.Body(%#v) error: %v", i, tt, r.err)
			continue
		}
		if tt.headers != nil {
			for k, v := range tt.headers {
				if r.headers.Get(k) != v {
					t.Errorf("%d: r.headers[%q] = %q; want %q", i, k, v, v)
				}
			}
		}

		if r.body == nil {
			if len(tt.expected) != 0 {
				t.Errorf("%d: r.body = %q; want %q", i, r.body, tt.expected)
			}
			continue
		}
		buf := make([]byte, len(tt.expected))
		if _, err := r.body.Read(buf); err != nil {
			t.Errorf("%d: r.body.Read error: %v", i, err)
			continue
		}
		body := string(buf)
		if body != tt.expected {
			t.Errorf("%d: r.body = %q; want %q", i, body, tt.expected)
		}
	}
}

func TestWatch(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
	}{
		{
			name:       "no retry",
			maxRetries: 0,
		},
		{
			name:       "with retries",
			maxRetries: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var table = []struct {
				t   watch.EventType
				obj runtime.Object
			}{
				{watch.Added, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "first"}}},
				{watch.Modified, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "second"}}},
				{watch.Deleted, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "last"}}},
			}

			var attempts int
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() {
					attempts++
				}()

				flusher, ok := w.(http.Flusher)
				if !ok {
					panic("need flusher!")
				}

				if attempts < test.maxRetries {
					w.Header().Set("Retry-After", "1")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Transfer-Encoding", "chunked")
				w.WriteHeader(http.StatusOK)
				flusher.Flush()

				encoder := restclientwatch.NewEncoder(streaming.NewEncoder(w, jsonEncoder(t)), jsonEncoder(t))
				for _, item := range table {
					if err := encoder.Encode(&watch.Event{Type: item.t, Object: item.obj}); err != nil {
						panic(err)
					}
					flusher.Flush()
				}
			}))
			defer testServer.Close()

			s := testRESTClient(t, testServer)
			watching, err := s.Get().
				ApiPath("/api").
				GroupVersion(v1.SchemeGroupVersion).
				Prefix("path/to/watch/thing").
				MaxRetries(test.maxRetries).
				Watch(context.Background())

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			outStream, _, cancelListen := concurrent.Watch(context.Background(), watching)
			defer cancelListen()

			for _, item := range table {
				got, ok := <-outStream
				if !ok {
					t.Fatalf("Unexpected early close")
				}
				if e, a := item.t, got.Type; e != a {
					t.Errorf("Expected %v, got %v", e, a)
				}
				if e, a := item.obj, got.Object; !apiequality.Semantic.DeepDerivative(e, a) {
					t.Errorf("Expected %v, got %v", e, a)
				}
			}

			_, ok := <-outStream
			if ok {
				t.Fatal("Unexpected non-close")
			}
		})
	}
}

func TestWatchNonDefaultContentType(t *testing.T) {
	var table = []struct {
		t   watch.EventType
		obj runtime.Object
	}{
		{watch.Added, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "first"}}},
		{watch.Modified, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "second"}}},
		{watch.Deleted, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "last"}}},
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			panic("need flusher!")
		}

		w.Header().Set("Transfer-Encoding", "chunked")
		// manually set the content type here so we get the renegotiation behavior
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		encoder := restclientwatch.NewEncoder(streaming.NewEncoder(w, jsonEncoder(t)), jsonEncoder(t))
		for _, item := range table {
			if err := encoder.Encode(&watch.Event{Type: item.t, Object: item.obj}); err != nil {
				panic(err)
			}
			flusher.Flush()
		}
	}))
	defer testServer.Close()

	// set the default content type to protobuf so that we test falling back to JSON serialization
	ClientContentConfig := newNegotiator(t)
	// ClientContentConfig.ContentType = "application/vnd.kubernetes.protobuf"
	s := testRESTClientWithConfig(t, testServer, ClientContentConfig)
	watching, err := s.Get().Prefix("path/to/watch/thing").Watch(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error")
	}

	outStream, _, cancelListen := concurrent.Watch(context.Background(), watching)
	defer cancelListen()

	for _, item := range table {
		got, ok := <-outStream
		if !ok {
			t.Fatalf("Unexpected early close")
		}
		if e, a := item.t, got.Type; e != a {
			t.Errorf("Expected %v, got %v", e, a)
		}
		if e, a := item.obj, got.Object; !apiequality.Semantic.DeepDerivative(e, a) {
			t.Errorf("Expected %v, got %v", e, a)
		}
	}

	_, ok := <-outStream
	if ok {
		t.Fatal("Unexpected non-close")
	}
}

func TestWatchUnknownContentType(t *testing.T) {
	var table = []struct {
		t   watch.EventType
		obj runtime.Object
	}{
		{watch.Added, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "first"}}},
		{watch.Modified, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "second"}}},
		{watch.Deleted, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "last"}}},
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			panic("need flusher!")
		}

		w.Header().Set("Transfer-Encoding", "chunked")
		// manually set the content type here so we get the renegotiation behavior
		w.Header().Set("Content-Type", "foobar")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		encoder := restclientwatch.NewEncoder(streaming.NewEncoder(w, jsonEncoder(t)), jsonEncoder(t))
		for _, item := range table {
			if err := encoder.Encode(&watch.Event{Type: item.t, Object: item.obj}); err != nil {
				panic(err)
			}
			flusher.Flush()
		}
	}))
	defer testServer.Close()

	s := testRESTClient(t, testServer)
	_, err := s.Get().Prefix("path/to/watch/thing").Watch(context.Background())
	if err == nil {
		t.Fatalf("Expected to fail due to lack of known stream serialization for content type")
	}
}

func TestStream(t *testing.T) {
	expectedBody := "expected body"

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			panic("need flusher!")
		}
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
		flusher.Flush()
	}))
	defer testServer.Close()

	s := testRESTClient(t, testServer)
	readCloser, err := s.Get().Prefix("path/to/stream/thing").Stream(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer readCloser.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(readCloser)
	resultBody := buf.String()

	if expectedBody != resultBody {
		t.Errorf("Expected %s, got %s", expectedBody, resultBody)
	}
}

func testRESTClientWithConfig(t testing.TB, srv *httptest.Server, SerializerNegotiator SerializerNegotiator) *RESTClient {
	base, _ := url.Parse("http://localhost")
	var c *http.Client
	if srv != nil {
		var err error
		base, err = url.Parse(srv.URL)
		if err != nil {
			t.Fatalf("failed to parse test URL: %v", err)
		}
		c = srv.Client()
	}
	client, err := NewRESTClient(base, SerializerNegotiator, nil, nil, nil, c)
	if err != nil {
		t.Fatalf("failed to create a client: %v", err)
	}
	return client
}

func testRESTClient(t testing.TB, srv *httptest.Server) *RESTClient {
	ClientContentConfig := newNegotiator(t)
	return testRESTClientWithConfig(t, srv, ClientContentConfig)
}

func TestDoContext(t *testing.T) {
	receivedCh := make(chan struct{})
	block := make(chan struct{})
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(receivedCh)
		<-block
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()
	defer close(block)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-receivedCh
		cancel()
	}()

	c := testRESTClient(t, testServer)
	_, err := c.Verb("GET").
		Prefix("foo").
		DoRaw(ctx)
	if err == nil {
		t.Fatal("Expected context cancellation error")
	}
}

func buildString(length int) string {
	s := make([]byte, length)
	for i := range s {
		s[i] = 'a'
	}
	return string(s)
}

func init() {
	klog.InitFlags(nil)
}

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		body  string
		want  string
		level string
	}{
		// Anything below 8 is completely truncated
		{
			body:  "Completely truncated below 8",
			want:  " [truncated 28 chars]",
			level: "0",
		},
		// Small strings are not truncated by high levels
		{
			body:  "Small body never gets truncated",
			want:  "Small body never gets truncated",
			level: "10",
		},
		{
			body:  "Small body never gets truncated",
			want:  "Small body never gets truncated",
			level: "8",
		},
		// Strings are truncated to 1024 if level is less than 9.
		{
			body:  buildString(2000),
			level: "8",
			want:  fmt.Sprintf("%s [truncated 976 chars]", buildString(1024)),
		},
		// Strings are truncated to 10240 if level is 9.
		{
			body:  buildString(20000),
			level: "9",
			want:  fmt.Sprintf("%s [truncated 9760 chars]", buildString(10240)),
		},
		// Strings are not truncated if level is 10 or higher
		{
			body:  buildString(20000),
			level: "10",
			want:  buildString(20000),
		},
		// Strings are not truncated if level is 10 or higher
		{
			body:  buildString(20000),
			level: "11",
			want:  buildString(20000),
		},
	}

	l := flag.Lookup("v").Value.(flag.Getter).Get().(klog.Level)
	for _, test := range tests {
		flag.Set("v", test.level)
		got := truncateBody(test.body)
		if got != test.want {
			t.Errorf("truncateBody(%v) = %v, want %v", test.body, got, test.want)
		}
	}
	flag.Set("v", l.String())
}

func defaultResourcePathWithPrefix(prefix, resource, namespace, name string) string {
	var path string
	path = "/api/" + v1.SchemeGroupVersion.Version

	if prefix != "" {
		path = path + "/" + prefix
	}
	if namespace != "" {
		path = path + "/namespaces/" + namespace
	}
	// Resource names are lower case.
	resource = strings.ToLower(resource)
	if resource != "" {
		path = path + "/" + resource
	}
	if name != "" {
		path = path + "/" + name
	}
	return path
}

func TestRequestPreflightCheck(t *testing.T) {
	for _, tt := range []struct {
		name         string
		verbs        []string
		namespace    string
		resourceName string
		namespaceSet bool
		expectsErr   bool
	}{
		{
			name:         "no namespace set",
			verbs:        []string{"GET", "PUT", "DELETE", "POST"},
			namespaceSet: false,
			expectsErr:   false,
		},
		{
			name:         "empty resource name and namespace",
			verbs:        []string{"GET", "PUT", "DELETE"},
			namespaceSet: true,
			expectsErr:   false,
		},
		{
			name:         "resource name with empty namespace",
			verbs:        []string{"GET", "PUT", "DELETE"},
			namespaceSet: true,
			resourceName: "ResourceName",
			expectsErr:   true,
		},
		{
			name:         "post empty resource name and namespace",
			verbs:        []string{"POST"},
			namespaceSet: true,
			expectsErr:   true,
		},
		{
			name:         "working requests",
			verbs:        []string{"GET", "PUT", "DELETE", "POST"},
			namespaceSet: true,
			resourceName: "ResourceName",
			namespace:    "Namespace",
			expectsErr:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, verb := range tt.verbs {
				r := &Request{
					verb:         verb,
					namespace:    tt.namespace,
					resourceName: tt.resourceName,
					namespaceSet: tt.namespaceSet,
				}

				err := r.requestPreflightCheck()
				hasErr := err != nil
				if hasErr == tt.expectsErr {
					return
				}
				t.Errorf("%s: expects error: %v, has error: %v", verb, tt.expectsErr, hasErr)
			}
		})
	}
}

func TestThrottledLogger(t *testing.T) {
	now := time.Now()
	oldClock := globalThrottledLogger.clock
	defer func() {
		globalThrottledLogger.clock = oldClock
	}()
	clock := testingclock.NewFakeClock(now)
	globalThrottledLogger.clock = clock

	logMessages := 0
	for i := 0; i < 1000; i++ {
		var wg sync.WaitGroup
		wg.Add(10)
		for j := 0; j < 10; j++ {
			go func() {
				if _, ok := globalThrottledLogger.attemptToLog(); ok {
					logMessages++
				}
				wg.Done()
			}()
		}
		wg.Wait()
		now = now.Add(1 * time.Second)
		clock.SetTime(now)
	}

	if a, e := logMessages, 100; a != e {
		t.Fatalf("expected %v log messages, but got %v", e, a)
	}
}

func TestRequestMaxRetries(t *testing.T) {
	successAtNthCalls := 1
	actualCalls := 0
	retryOneTimeHandler := func(w http.ResponseWriter, req *http.Request) {
		defer func() { actualCalls++ }()
		if actualCalls >= successAtNthCalls {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		actualCalls++
	}
	testServer := httptest.NewServer(http.HandlerFunc(retryOneTimeHandler))
	defer testServer.Close()

	u, err := url.Parse(testServer.URL)
	if err != nil {
		t.Error(err)
	}

	testCases := []struct {
		name        string
		maxRetries  int
		expectError bool
	}{
		{
			name:        "no retrying should fail",
			maxRetries:  0,
			expectError: true,
		},
		{
			name:        "1 max-retry should exactly work",
			maxRetries:  1,
			expectError: false,
		},
		{
			name:        "5 max-retry should work",
			maxRetries:  5,
			expectError: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			defer func() { actualCalls = 0 }()
			_, err := NewRequestWithClient(u, newNegotiator(t), testServer.Client()).
				Verb("get").
				MaxRetries(testCase.maxRetries).
				AbsPath("/foo").
				DoRaw(context.TODO())
			hasError := err != nil
			if testCase.expectError != hasError {
				t.Error(" failed checking error")
			}
		})
	}
}

type responseErr struct {
	response *http.Response
	err      error
}

type seek struct {
	offset int64
	whence int
}

type count struct {
	// keeps track of the number of Seek(offset, whence) calls.
	seeks []seek

	// how many times {Request|Response}.Body.Close() has been invoked
	lock   sync.Mutex
	closes int
}

func (c *count) close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.closes++
}
func (c *count) getCloseCount() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.closes
}

// used to track {Request|Response}.Body
type readTracker struct {
	delegated io.Reader
	count     *count
}

func (r *readTracker) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := r.delegated.(io.Seeker); ok {
		r.count.seeks = append(r.count.seeks, seek{offset: offset, whence: whence})
		return seeker.Seek(offset, whence)
	}
	return 0, io.EOF
}

func (r *readTracker) Read(p []byte) (n int, err error) {
	return r.delegated.Read(p)
}

func (r *readTracker) Close() error {
	if closer, ok := r.delegated.(io.Closer); ok {
		r.count.close()
		return closer.Close()
	}
	return nil
}

func newReadTracker(count *count) *readTracker {
	return &readTracker{
		count: count,
	}
}

func newCount() *count {
	return &count{
		closes: 0,
		seeks:  make([]seek, 0),
	}
}

type readSeeker struct{ err error }

func (rs readSeeker) Read([]byte) (int, error)       { return 0, rs.err }
func (rs readSeeker) Seek(int64, int) (int64, error) { return 0, rs.err }

func unWrap(err error) error {
	if uerr, ok := err.(*url.Error); ok {
		return uerr.Err
	}
	return err
}

// noSleepBackOff is a NoBackoff except it does not sleep,
// used for faster execution of the unit tests.
type noSleepBackOff struct {
	*NoBackoff
}

func (n *noSleepBackOff) Sleep(d time.Duration) {}

func TestRequestWithRetry(t *testing.T) {
	tests := []struct {
		name                         string
		body                         io.Reader
		serverReturns                responseErr
		errExpected                  error
		errContains                  string
		transformFuncInvokedExpected int
		roundTripInvokedExpected     int
	}{
		{
			name:                         "server returns retry-after response, request body is not io.Seeker, retry goes ahead",
			body:                         ioutil.NopCloser(bytes.NewReader([]byte{})),
			serverReturns:                responseErr{response: retryAfterResponse(), err: nil},
			errExpected:                  nil,
			transformFuncInvokedExpected: 1,
			roundTripInvokedExpected:     2,
		},
		{
			name:                         "server returns retry-after response, request body Seek returns error, retry aborted",
			body:                         &readSeeker{err: io.EOF},
			serverReturns:                responseErr{response: retryAfterResponse(), err: nil},
			errExpected:                  nil,
			transformFuncInvokedExpected: 0,
			roundTripInvokedExpected:     1,
		},
		{
			name:                         "server returns retry-after response, request body Seek returns no error, retry goes ahead",
			body:                         &readSeeker{err: nil},
			serverReturns:                responseErr{response: retryAfterResponse(), err: nil},
			errExpected:                  nil,
			transformFuncInvokedExpected: 1,
			roundTripInvokedExpected:     2,
		},
		{
			name:                         "server returns retryable err, request body is not io.Seek, retry goes ahead",
			body:                         ioutil.NopCloser(bytes.NewReader([]byte{})),
			serverReturns:                responseErr{response: nil, err: io.ErrUnexpectedEOF},
			errExpected:                  io.ErrUnexpectedEOF,
			transformFuncInvokedExpected: 0,
			roundTripInvokedExpected:     2,
		},
		{
			name:                         "server returns retryable err, request body Seek returns error, retry aborted",
			body:                         &readSeeker{err: io.EOF},
			serverReturns:                responseErr{response: nil, err: io.ErrUnexpectedEOF},
			errContains:                  "failed to reset the request body while retrying a request: EOF",
			transformFuncInvokedExpected: 0,
			roundTripInvokedExpected:     1,
		},
		{
			name:                         "server returns retryable err, request body Seek returns no err, retry goes ahead",
			body:                         &readSeeker{err: nil},
			serverReturns:                responseErr{response: nil, err: io.ErrUnexpectedEOF},
			errExpected:                  io.ErrUnexpectedEOF,
			transformFuncInvokedExpected: 0,
			roundTripInvokedExpected:     2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var roundTripInvoked int
			client := clientForFunc(func(req *http.Request) (*http.Response, error) {
				roundTripInvoked++
				return test.serverReturns.response, test.serverReturns.err
			})

			req := &Request{
				verb: "GET",
				body: test.body,
				c: &RESTClient{
					Client: client,
				},
				backoff:    &noSleepBackOff{},
				maxRetries: 1,
				retryFn:    defaultRequestRetryFn,
			}

			var transformFuncInvoked int
			err := req.request(context.Background(), func(request *http.Request, response *http.Response) {
				transformFuncInvoked++
			})

			if test.roundTripInvokedExpected != roundTripInvoked {
				t.Errorf("Expected RoundTrip to be invoked %d times, but got: %d", test.roundTripInvokedExpected, roundTripInvoked)
			}
			if test.transformFuncInvokedExpected != transformFuncInvoked {
				t.Errorf("Expected transform func to be invoked %d times, but got: %d", test.transformFuncInvokedExpected, transformFuncInvoked)
			}
			switch {
			case test.errExpected != nil:
				if test.errExpected != unWrap(err) {
					t.Errorf("Expected error: %v, but got: %v", test.errExpected, unWrap(err))
				}
			case len(test.errContains) > 0:
				if !strings.Contains(err.Error(), test.errContains) {
					t.Errorf("Expected error message to caontain: %q, but got: %q", test.errContains, err.Error())
				}
			}
		})
	}
}

func TestRequestDoWithRetry(t *testing.T) {
	testRequestWithRetry(t, "Do", func(ctx context.Context, r *Request) {
		r.Do(ctx)
	})
}

func TestRequestDoRawWithRetry(t *testing.T) {
	// both request.Do and request.DoRaw have the same behavior and expectations
	testRequestWithRetry(t, "Do", func(ctx context.Context, r *Request) {
		r.DoRaw(ctx)
	})
}

func TestRequestStreamWithRetry(t *testing.T) {
	testRequestWithRetry(t, "Stream", func(ctx context.Context, r *Request) {
		r.Stream(ctx)
	})
}

func TestRequestWatchWithRetry(t *testing.T) {
	testRequestWithRetry(t, "Watch", func(ctx context.Context, r *Request) {
		w, err := r.Watch(ctx)
		if err == nil {
			// in this test the the response body returned by the server is always empty,
			// this will cause StreamWatcher.receive() to:
			// - return an io.EOF to indicate that the watch closed normally and
			// - then close the io.Reader
			// since we assert on the number of times 'Close' has been called on the
			// body of the response object, we need to wait here to avoid race condition.
			outStream, _, complete := concurrent.Watch(ctx, w)
			defer complete()
			<-outStream
		}
	})
}

func TestRequestDoRetryWithRateLimiterBackoffAndMetrics(t *testing.T) {
	// both request.Do and request.DoRaw have the same behavior and expectations
	testRetryWithRateLimiterBackoffAndMetrics(t, "Do", func(ctx context.Context, r *Request) {
		r.DoRaw(ctx)
	})
}

func TestRequestStreamRetryWithRateLimiterBackoffAndMetrics(t *testing.T) {
	testRetryWithRateLimiterBackoffAndMetrics(t, "Stream", func(ctx context.Context, r *Request) {
		r.Stream(ctx)
	})
}

func TestRequestWatchRetryWithRateLimiterBackoffAndMetrics(t *testing.T) {
	testRetryWithRateLimiterBackoffAndMetrics(t, "Watch", func(ctx context.Context, r *Request) {
		w, err := r.Watch(ctx)
		if err == nil {
			// in this test the the response body returned by the server is always empty,
			// this will cause StreamWatcher.receive() to:
			// - return an io.EOF to indicate that the watch closed normally and
			// - then close the io.Reader
			// since we assert on the number of times 'Close' has been called on the
			// body of the response object, we need to wait here to avoid race condition.
			outStream, _, complete := concurrent.Watch(ctx, w)
			defer complete()
			<-outStream
		}
	})
}

func TestRequestDoWithRetryInvokeOrder(t *testing.T) {
	// both request.Do and request.DoRaw have the same behavior and expectations
	testWithRetryInvokeOrder(t, "Do", func(ctx context.Context, r *Request) {
		r.DoRaw(ctx)
	})
}

func TestRequestStreamWithRetryInvokeOrder(t *testing.T) {
	testWithRetryInvokeOrder(t, "Stream", func(ctx context.Context, r *Request) {
		r.Stream(ctx)
	})
}

func TestRequestWatchWithRetryInvokeOrder(t *testing.T) {
	testWithRetryInvokeOrder(t, "Watch", func(ctx context.Context, r *Request) {
		w, err := r.Watch(ctx)
		if err == nil {
			// in this test the the response body returned by the server is always empty,
			// this will cause StreamWatcher.receive() to:
			// - return an io.EOF to indicate that the watch closed normally and
			// - then close the io.Reader
			// since we assert on the number of times 'Close' has been called on the
			// body of the response object, we need to wait here to avoid race condition.
			outStream, _, complete := concurrent.Watch(ctx, w)
			defer complete()
			<-outStream
		}
	})
}

func TestRequestWatchWithWrapPreviousError(t *testing.T) {
	testWithWrapPreviousError(t, func(ctx context.Context, r *Request) error {
		w, err := r.Watch(ctx)
		if err == nil {
			// in this test the the response body returned by the server is always empty,
			// this will cause StreamWatcher.receive() to:
			// - return an io.EOF to indicate that the watch closed normally and
			// - then close the io.Reader
			// since we assert on the number of times 'Close' has been called on the
			// body of the response object, we need to wait here to avoid race condition.
			outStream, _, complete := concurrent.Watch(ctx, w)
			defer complete()
			<-outStream
		}
		return err
	})
}

func TestRequestDoWithWrapPreviousError(t *testing.T) {
	// both request.Do and request.DoRaw have the same behavior and expectations
	testWithWrapPreviousError(t, func(ctx context.Context, r *Request) error {
		result := r.Do(ctx)
		return result.err
	})
}

func testRequestWithRetry(t *testing.T, key string, doFunc func(ctx context.Context, r *Request)) {
	type expected struct {
		attempts  int
		reqCount  *count
		respCount *count
	}

	tests := []struct {
		name          string
		verb          string
		body          func() io.Reader
		maxRetries    int
		serverReturns []responseErr

		// expectations differ based on whether it is 'Watch', 'Stream' or 'Do'
		expectations map[string]expected
	}{
		{
			name:       "server always returns retry-after response",
			verb:       "GET",
			body:       func() io.Reader { return bytes.NewReader([]byte{}) },
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
			},
			expectations: map[string]expected{
				"Do": {
					attempts:  3,
					reqCount:  &count{closes: 0, seeks: make([]seek, 2)},
					respCount: &count{closes: 3, seeks: []seek{}},
				},
				"Watch": {
					attempts:  3,
					reqCount:  &count{closes: 0, seeks: make([]seek, 2)},
					respCount: &count{closes: 3, seeks: []seek{}},
				},
				"Stream": {
					attempts:  3,
					reqCount:  &count{closes: 0, seeks: make([]seek, 2)},
					respCount: &count{closes: 3, seeks: []seek{}},
				},
			},
		},
		{
			name:       "server always returns retryable error",
			verb:       "GET",
			body:       func() io.Reader { return bytes.NewReader([]byte{}) },
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: nil, err: io.EOF},
				{response: nil, err: io.EOF},
				{response: nil, err: io.EOF},
			},
			expectations: map[string]expected{
				"Do": {
					attempts:  3,
					reqCount:  &count{closes: 0, seeks: make([]seek, 2)},
					respCount: &count{closes: 0, seeks: []seek{}},
				},
				"Watch": {
					attempts:  3,
					reqCount:  &count{closes: 0, seeks: make([]seek, 2)},
					respCount: &count{closes: 0, seeks: []seek{}},
				},
				// for Stream, we never retry on any error
				"Stream": {
					attempts:  1, // only the first attempt is expected
					reqCount:  &count{closes: 0, seeks: []seek{}},
					respCount: &count{closes: 0, seeks: []seek{}},
				},
			},
		},
		{
			name:       "server returns success on the final retry",
			verb:       "GET",
			body:       func() io.Reader { return bytes.NewReader([]byte{}) },
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: nil, err: io.EOF},
				{response: &http.Response{StatusCode: http.StatusOK}, err: nil},
			},
			expectations: map[string]expected{
				"Do": {
					attempts:  3,
					reqCount:  &count{closes: 0, seeks: make([]seek, 2)},
					respCount: &count{closes: 2, seeks: []seek{}},
				},
				"Watch": {
					attempts: 3,
					reqCount: &count{closes: 0, seeks: make([]seek, 2)},
					// the Body of the successful response object will get closed by
					// StreamWatcher, so we need to take that into account.
					respCount: &count{closes: 2, seeks: []seek{}},
				},
				"Stream": {
					attempts:  2,
					reqCount:  &count{closes: 0, seeks: make([]seek, 1)},
					respCount: &count{closes: 1, seeks: []seek{}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			respCountGot := newCount()
			responseRecorder := newReadTracker(respCountGot)
			var attempts int
			client := clientForFunc(func(req *http.Request) (*http.Response, error) {
				defer func() {
					attempts++
				}()

				resp := test.serverReturns[attempts].response
				if resp != nil {
					responseRecorder.delegated = ioutil.NopCloser(bytes.NewReader([]byte{}))
					resp.Body = responseRecorder
				}
				return resp, test.serverReturns[attempts].err
			})

			reqCountGot := newCount()
			reqRecorder := newReadTracker(reqCountGot)
			reqRecorder.delegated = test.body()

			req := &Request{
				verb: test.verb,
				body: reqRecorder,
				c: &RESTClient{
					negotiator: newNegotiator(t),
					Client:     client,
				},
				backoff:    &noSleepBackOff{},
				maxRetries: test.maxRetries,
				retryFn:    defaultRequestRetryFn,
			}

			doFunc(context.Background(), req)

			expected, ok := test.expectations[key]
			if !ok {
				t.Fatalf("Wrong test setup - did not find expected for: %s", key)
			}
			if expected.attempts != attempts {
				t.Errorf("Expected retries: %d, but got: %d", expected.attempts, attempts)
			}

			if !reflect.DeepEqual(expected.reqCount.seeks, reqCountGot.seeks) {
				t.Errorf("Expected request body to have seek invocation: %v, but got: %v", expected.reqCount.seeks, reqCountGot.seeks)
			}
			if expected.respCount.closes != respCountGot.getCloseCount() {
				t.Errorf("Expected response body Close to be invoked %d times, but got: %d", expected.respCount.closes, respCountGot.getCloseCount())
			}
		})
	}
}

type retryTestKeyType int

const retryTestKey retryTestKeyType = iota

// fake flowcontrol.RateLimiter so we can tap into the Wait method of the rate limiter.
// fake BackoffManager so we can tap into backoff calls
// fake metrics.ResultMetric to tap into the metric calls
// we use it to verify that RateLimiter, BackoffManager, and
// metric calls are invoked appropriately in right order.
type withRateLimiterBackoffManagerAndMetrics struct {
	flowcontrol.RateLimiter
	*NoBackoff
	metrics.ResultMetric
	calculateBackoffSeq int64
	calculateBackoffFn  func(i int64) time.Duration

	invokeOrderGot []string
	sleepsGot      []string
	statusCodesGot []string
}

func (lb *withRateLimiterBackoffManagerAndMetrics) Wait(ctx context.Context) error {
	lb.invokeOrderGot = append(lb.invokeOrderGot, "RateLimiter.Wait")
	return nil
}

func (lb *withRateLimiterBackoffManagerAndMetrics) CalculateBackoff(actualUrl *url.URL) time.Duration {
	lb.invokeOrderGot = append(lb.invokeOrderGot, "BackoffManager.CalculateBackoff")

	waitFor := lb.calculateBackoffFn(lb.calculateBackoffSeq)
	lb.calculateBackoffSeq++
	return waitFor
}

func (lb *withRateLimiterBackoffManagerAndMetrics) UpdateBackoff(actualUrl *url.URL, err error, responseCode int) {
	lb.invokeOrderGot = append(lb.invokeOrderGot, "BackoffManager.UpdateBackoff")
}

func (lb *withRateLimiterBackoffManagerAndMetrics) Sleep(d time.Duration) {
	lb.invokeOrderGot = append(lb.invokeOrderGot, "BackoffManager.Sleep")
	lb.sleepsGot = append(lb.sleepsGot, d.String())
}

func (lb *withRateLimiterBackoffManagerAndMetrics) Increment(ctx context.Context, code, _, _ string) {
	// we are interested in the request context that is marked by this test
	if marked, ok := ctx.Value(retryTestKey).(bool); ok && marked {
		lb.invokeOrderGot = append(lb.invokeOrderGot, "RequestResult.Increment")
		lb.statusCodesGot = append(lb.statusCodesGot, code)
	}
}

func (lb *withRateLimiterBackoffManagerAndMetrics) Do() {
	lb.invokeOrderGot = append(lb.invokeOrderGot, "Client.Do")
}

func testRetryWithRateLimiterBackoffAndMetrics(t *testing.T, key string, doFunc func(ctx context.Context, r *Request)) {
	type expected struct {
		attempts    int
		order       []string
		sleeps      []string
		statusCodes []string
	}

	// we define the expected order of how the client invokes the
	// rate limiter, backoff, and metrics methods.
	// scenario:
	//  - A: original request fails with a retryable response: (500, 'Retry-After: N')
	//  - B: retry 1: successful with a status code 200
	// so we have a total of 2 attempts
	invokeOrderWant := []string{
		// before we send the request to the server:
		// - we wait as dictated by the client rate lmiter
		// - we wait, as dictated by the backoff manager
		"RateLimiter.Wait",
		"BackoffManager.CalculateBackoff",
		"BackoffManager.Sleep",

		// A: first attempt for which the server sends a retryable response
		// status code: 500, Retry-Afer: N
		"Client.Do",

		// we got a response object, status code: 500, Retry-Afer: N
		//  - call metrics method with appropriate status code
		//  - update backoff parameters with the status code returned
		"RequestResult.Increment",
		"BackoffManager.UpdateBackoff",
		"BackoffManager.CalculateBackoff",
		// sleep for delay=max(BackoffManager.CalculateBackoff, Retry-After: N)
		"BackoffManager.Sleep",
		// wait as dictated by the client rate lmiter
		"RateLimiter.Wait",

		// B: 2nd attempt: retry, and this should return a status code=200
		"Client.Do",

		// it's a success, so do the following:
		//  - call metrics and update backoff parameters
		"RequestResult.Increment",
		"BackoffManager.UpdateBackoff",
	}
	statusCodesWant := []string{
		"500",
		"200",
	}

	tests := []struct {
		name               string
		maxRetries         int
		serverReturns      []responseErr
		calculateBackoffFn func(i int64) time.Duration
		// expectations differ based on whether it is 'Watch', 'Stream' or 'Do'
		expectations map[string]expected
	}{
		{
			name:       "success after one retry, Retry-After: N > BackoffManager.CalculateBackoff",
			maxRetries: 1,
			serverReturns: []responseErr{
				{response: retryAfterResponseWithDelay("5"), err: nil},
				{response: &http.Response{StatusCode: http.StatusOK}, err: nil},
			},
			// we simulate a sleep sequence of 0s, 1s, 2s, 3s, ...
			calculateBackoffFn: func(i int64) time.Duration { return time.Duration(i * int64(time.Second)) },
			expectations: map[string]expected{
				"Do": {
					attempts:    2,
					order:       invokeOrderWant,
					statusCodes: statusCodesWant,
					sleeps: []string{
						// initial backoff.Sleep before we send the request to the server for the first time
						"0s",
						// maximum of:
						//  - 'Retry-After: 5' response header from (A)
						//  - BackoffManager.CalculateBackoff (will return 1s)
						(5 * time.Second).String(),
					},
				},
				"Watch": {
					attempts: 2,
					// Watch does not do 'RateLimiter.Wait' before initially sending the request to the server
					order:       invokeOrderWant[1:],
					statusCodes: statusCodesWant,
					sleeps: []string{
						"0s",
						(5 * time.Second).String(),
					},
				},
				"Stream": {
					attempts:    2,
					order:       invokeOrderWant,
					statusCodes: statusCodesWant,
					sleeps: []string{
						"0s",
						(5 * time.Second).String(),
					},
				},
			},
		},
		{
			name:       "success after one retry, Retry-After: N < BackoffManager.CalculateBackoff",
			maxRetries: 1,
			serverReturns: []responseErr{
				{response: retryAfterResponseWithDelay("2"), err: nil},
				{response: &http.Response{StatusCode: http.StatusOK}, err: nil},
			},
			// we simulate a sleep sequence of 0s, 4s, 8s, 16s, ...
			calculateBackoffFn: func(i int64) time.Duration { return time.Duration(i * int64(4*time.Second)) },
			expectations: map[string]expected{
				"Do": {
					attempts:    2,
					order:       invokeOrderWant,
					statusCodes: statusCodesWant,
					sleeps: []string{
						// initial backoff.Sleep before we send the request to the server for the first time
						"0s",
						// maximum of:
						//  - 'Retry-After: 2' response header from (A)
						//  - BackoffManager.CalculateBackoff (will return 4s)
						(4 * time.Second).String(),
					},
				},
				"Watch": {
					attempts: 2,
					// Watch does not do 'RateLimiter.Wait' before initially sending the request to the server
					order:       invokeOrderWant[1:],
					statusCodes: statusCodesWant,
					sleeps: []string{
						"0s",
						(4 * time.Second).String(),
					},
				},
				"Stream": {
					attempts:    2,
					order:       invokeOrderWant,
					statusCodes: statusCodesWant,
					sleeps: []string{
						"0s",
						(4 * time.Second).String(),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interceptor := &withRateLimiterBackoffManagerAndMetrics{
				RateLimiter:        flowcontrol.NewFakeAlwaysRateLimiter(),
				NoBackoff:          &NoBackoff{},
				calculateBackoffFn: test.calculateBackoffFn,
			}

			// TODO: today this is the only site where a test overrides the
			//  default metric interfaces, in future if we other tests want
			//  to override as well, and we want tests to be able to run in
			//  parallel then we will need to provide a way for tests to
			//  register/deregister their own metric inerfaces.
			old := metrics.RequestResult
			metrics.RequestResult = interceptor
			defer func() {
				metrics.RequestResult = old
			}()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// we are changing metrics.RequestResult (a global state) in
			// this test, to avoid interference from other tests running in
			// parallel we need to associate a key to the context so we
			// can identify the metric calls associated with this test.
			ctx = context.WithValue(ctx, retryTestKey, true)

			var attempts int
			client := clientForFunc(func(req *http.Request) (*http.Response, error) {
				defer func() {
					attempts++
				}()

				interceptor.Do()
				resp := test.serverReturns[attempts].response
				if resp != nil {
					resp.Body = ioutil.NopCloser(bytes.NewReader([]byte{}))
				}
				return resp, test.serverReturns[attempts].err
			})

			base, err := url.Parse("http://foo.bar")
			if err != nil {
				t.Fatalf("Wrong test setup - did not find expected for: %s", key)
			}
			req := &Request{
				verb: "GET",
				body: bytes.NewReader([]byte{}),
				c: &RESTClient{
					base:        base,
					negotiator:  newNegotiator(t),
					Client:      client,
					rateLimiter: interceptor,
				},
				pathPrefix:  "/api/v1",
				rateLimiter: interceptor,
				backoff:     interceptor,
				maxRetries:  test.maxRetries,
				retryFn:     defaultRequestRetryFn,
			}

			doFunc(ctx, req)

			want, ok := test.expectations[key]
			if !ok {
				t.Fatalf("Wrong test setup - did not find expected for: %s", key)
			}
			if want.attempts != attempts {
				t.Errorf("%s: Expected retries: %d, but got: %d", key, want.attempts, attempts)
			}
			if !cmp.Equal(want.order, interceptor.invokeOrderGot) {
				t.Errorf("%s: Expected invoke order to match, diff: %s", key, cmp.Diff(want.order, interceptor.invokeOrderGot))
			}
			if !cmp.Equal(want.sleeps, interceptor.sleepsGot) {
				t.Errorf("%s: Expected sleep sequence to match, diff: %s", key, cmp.Diff(want.sleeps, interceptor.sleepsGot))
			}
			if !cmp.Equal(want.statusCodes, interceptor.statusCodesGot) {
				t.Errorf("%s: Expected status codes to match, diff: %s", key, cmp.Diff(want.statusCodes, interceptor.statusCodesGot))
			}
		})
	}
}

type retryInterceptor struct {
	WithRetry
	invokeOrderGot []string
}

func (ri *retryInterceptor) IsNextRetry(ctx context.Context, restReq *Request, httpReq *http.Request, resp *http.Response, err error, f IsRetryableErrorFunc) bool {
	ri.invokeOrderGot = append(ri.invokeOrderGot, "WithRetry.IsNextRetry")
	return ri.WithRetry.IsNextRetry(ctx, restReq, httpReq, resp, err, f)
}

func (ri *retryInterceptor) Before(ctx context.Context, request *Request) error {
	ri.invokeOrderGot = append(ri.invokeOrderGot, "WithRetry.Before")
	return ri.WithRetry.Before(ctx, request)
}

func (ri *retryInterceptor) After(ctx context.Context, request *Request, resp *http.Response, err error) {
	ri.invokeOrderGot = append(ri.invokeOrderGot, "WithRetry.After")
	ri.WithRetry.After(ctx, request, resp, err)
}

func (ri *retryInterceptor) Do() {
	ri.invokeOrderGot = append(ri.invokeOrderGot, "Client.Do")
}

func testWithRetryInvokeOrder(t *testing.T, key string, doFunc func(ctx context.Context, r *Request)) {
	// we define the expected order of how the client
	// should invoke the retry interface
	// scenario:
	//  - A: original request fails with a retryable response: (500, 'Retry-After: 1')
	//  - B: retry 1: successful with a status code 200
	// so we have a total of 2 attempts
	defaultInvokeOrderWant := []string{
		// first attempt (A)
		"WithRetry.Before",
		"Client.Do",
		"WithRetry.After",
		// server returns a retryable response: (500, 'Retry-After: 1')
		// IsNextRetry is expected to return true
		"WithRetry.IsNextRetry",

		// second attempt (B) - retry 1: successful with a status code 200
		"WithRetry.Before",
		"Client.Do",
		"WithRetry.After",
		// success: IsNextRetry is expected to return false
		// Watch and Stream are an exception, they return as soon as the
		// server sends a status code of success.
		"WithRetry.IsNextRetry",
	}

	tests := []struct {
		name          string
		maxRetries    int
		serverReturns []responseErr
		// expectations differ based on whether it is 'Watch', 'Stream' or 'Do'
		expectations map[string][]string
	}{
		{
			name:       "success after one retry",
			maxRetries: 1,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: &http.Response{StatusCode: http.StatusOK}, err: nil},
			},
			expectations: map[string][]string{
				"Do": defaultInvokeOrderWant,
				// Watch and Stream skip the final 'IsNextRetry' by returning
				// as soon as they see a success from the server.
				"Watch":  defaultInvokeOrderWant[0 : len(defaultInvokeOrderWant)-1],
				"Stream": defaultInvokeOrderWant[0 : len(defaultInvokeOrderWant)-1],
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interceptor := &retryInterceptor{
				WithRetry: &withRetry{maxRetries: test.maxRetries},
			}

			var attempts int
			client := clientForFunc(func(req *http.Request) (*http.Response, error) {
				defer func() {
					attempts++
				}()

				interceptor.Do()
				resp := test.serverReturns[attempts].response
				if resp != nil {
					resp.Body = ioutil.NopCloser(bytes.NewReader([]byte{}))
				}
				return resp, test.serverReturns[attempts].err
			})

			base, err := url.Parse("http://foo.bar")
			if err != nil {
				t.Fatalf("Wrong test setup - did not find expected for: %s", key)
			}
			req := &Request{
				verb: "GET",
				body: bytes.NewReader([]byte{}),
				c: &RESTClient{
					base:       base,
					negotiator: newNegotiator(t),
					Client:     client,
				},
				pathPrefix:  "/api/v1",
				rateLimiter: flowcontrol.NewFakeAlwaysRateLimiter(),
				backoff:     &NoBackoff{},
				retryFn:     func(_ int) WithRetry { return interceptor },
			}

			doFunc(context.Background(), req)

			if attempts != 2 {
				t.Errorf("%s: Expected attempts: %d, but got: %d", key, 2, attempts)
			}
			invokeOrderWant, ok := test.expectations[key]
			if !ok {
				t.Fatalf("Wrong test setup - did not find expected for: %s", key)
			}
			if !cmp.Equal(invokeOrderWant, interceptor.invokeOrderGot) {
				t.Errorf("%s: Expected invoke order to match, diff: %s", key, cmp.Diff(invokeOrderWant, interceptor.invokeOrderGot))
			}
		})
	}
}

func testWithWrapPreviousError(t *testing.T, doFunc func(ctx context.Context, r *Request) error) {
	var (
		containsFormatExpected = "- error from a previous attempt: %s"
		nonRetryableErr        = errors.New("non retryable error")
	)

	tests := []struct {
		name             string
		maxRetries       int
		serverReturns    []responseErr
		expectedErr      error
		wrapped          bool
		attemptsExpected int
		contains         string
	}{
		{
			name:       "success at first attempt",
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: &http.Response{StatusCode: http.StatusOK}, err: nil},
			},
			attemptsExpected: 1,
			expectedErr:      nil,
		},
		{
			name:       "success after a series of retry-after from the server",
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: &http.Response{StatusCode: http.StatusOK}, err: nil},
			},
			attemptsExpected: 3,
			expectedErr:      nil,
		},
		{
			name:       "success after a series of retryable errors",
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: nil, err: io.EOF},
				{response: nil, err: io.EOF},
				{response: &http.Response{StatusCode: http.StatusOK}, err: nil},
			},
			attemptsExpected: 3,
			expectedErr:      nil,
		},
		{
			name:       "request errors out with a non retryable error",
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: nil, err: nonRetryableErr},
			},
			attemptsExpected: 1,
			expectedErr:      nonRetryableErr,
		},
		{
			name:       "request times out after retries, but no previous error",
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: nil, err: context.Canceled},
			},
			attemptsExpected: 3,
			expectedErr:      context.Canceled,
		},
		{
			name:       "request times out after retries, and we have a relevant previous error",
			maxRetries: 3,
			serverReturns: []responseErr{
				{response: nil, err: io.EOF},
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: nil, err: context.Canceled},
			},
			attemptsExpected: 4,
			wrapped:          true,
			expectedErr:      context.Canceled,
			contains:         fmt.Sprintf(containsFormatExpected, io.EOF),
		},
		{
			name:       "interleaved retry-after responses with retryable errors",
			maxRetries: 8,
			serverReturns: []responseErr{
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: nil, err: io.ErrUnexpectedEOF},
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: nil, err: io.EOF},
				{response: retryAfterResponse(), err: nil},
				{response: retryAfterResponse(), err: nil},
				{response: nil, err: context.Canceled},
			},
			attemptsExpected: 9,
			wrapped:          true,
			expectedErr:      context.Canceled,
			contains:         fmt.Sprintf(containsFormatExpected, io.EOF),
		},
		{
			name:       "request errors out with a retryable error, followed by a non-retryable one",
			maxRetries: 3,
			serverReturns: []responseErr{
				{response: nil, err: io.EOF},
				{response: nil, err: nonRetryableErr},
			},
			attemptsExpected: 2,
			wrapped:          true,
			expectedErr:      nonRetryableErr,
			contains:         fmt.Sprintf(containsFormatExpected, io.EOF),
		},
		{
			name:       "use the most recent error",
			maxRetries: 2,
			serverReturns: []responseErr{
				{response: nil, err: io.ErrUnexpectedEOF},
				{response: nil, err: io.EOF},
				{response: nil, err: context.Canceled},
			},
			attemptsExpected: 3,
			wrapped:          true,
			expectedErr:      context.Canceled,
			contains:         fmt.Sprintf(containsFormatExpected, io.EOF),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var attempts int
			client := clientForFunc(func(req *http.Request) (*http.Response, error) {
				defer func() {
					attempts++
				}()

				resp := test.serverReturns[attempts].response
				if resp != nil {
					resp.Body = ioutil.NopCloser(bytes.NewReader([]byte{}))
				}
				return resp, test.serverReturns[attempts].err
			})

			base, err := url.Parse("http://foo.bar")
			if err != nil {
				t.Fatalf("Failed to create new HTTP request - %v", err)
			}
			req := &Request{
				verb: "GET",
				body: bytes.NewReader([]byte{}),
				c: &RESTClient{
					base:       base,
					negotiator: newNegotiator(t),
					Client:     client,
				},
				pathPrefix:  "/api/v1",
				rateLimiter: flowcontrol.NewFakeAlwaysRateLimiter(),
				backoff:     &noSleepBackOff{},
				maxRetries:  test.maxRetries,
				retryFn:     defaultRequestRetryFn,
			}

			err = doFunc(context.Background(), req)
			if test.attemptsExpected != attempts {
				t.Errorf("Expected attempts: %d, but got: %d", test.attemptsExpected, attempts)
			}

			switch {
			case test.expectedErr == nil:
				if err != nil {
					t.Errorf("Expected a nil error, but got: %v", err)
					return
				}
			case test.expectedErr != nil:
				if !strings.Contains(err.Error(), test.contains) {
					t.Errorf("Expected error message to contain %q, but got: %v", test.contains, err)
				}

				urlErrGot, _ := err.(*url.Error)
				if test.wrapped {
					// we expect the url.Error from net/http to be wrapped by WrapPreviousError
					unwrapper, ok := err.(interface {
						Unwrap() error
					})
					if !ok {
						t.Errorf("Expected error to implement Unwrap method, but got: %v", err)
						return
					}
					urlErrGot, _ = unwrapper.Unwrap().(*url.Error)
				}
				// we always get a url.Error from net/http
				if urlErrGot == nil {
					t.Errorf("Expected error to be url.Error, but got: %v", err)
					return
				}

				errGot := urlErrGot.Unwrap()
				if test.expectedErr != errGot {
					t.Errorf("Expected error %v, but got: %v", test.expectedErr, errGot)
				}
			}
		})
	}
}

func TestReuseRequest(t *testing.T) {
	var tests = []struct {
		name        string
		enableHTTP2 bool
	}{
		{"HTTP1", false},
		{"HTTP2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(r.RemoteAddr))
			}))
			ts.EnableHTTP2 = tt.enableHTTP2
			ts.StartTLS()
			defer ts.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			c := testRESTClient(t, ts)

			req1, err := c.Verb("GET").
				Prefix("foo").
				DoRaw(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			req2, err := c.Verb("GET").
				Prefix("foo").
				DoRaw(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if string(req1) != string(req2) {
				t.Fatalf("Expected %v to be equal to %v", string(req1), string(req2))
			}

		})
	}
}

func TestHTTP1DoNotReuseRequestAfterTimeout(t *testing.T) {
	var tests = []struct {
		name        string
		enableHTTP2 bool
	}{
		{"HTTP1", false},
		{"HTTP2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Logf("TEST Connected from %v on %v\n", r.RemoteAddr, r.URL.Path)
				if r.URL.Path == "/hang" {
					t.Logf("TEST hanging %v\n", r.RemoteAddr)
					<-done
				}
				w.Write([]byte(r.RemoteAddr))
			}))
			ts.EnableHTTP2 = tt.enableHTTP2
			ts.StartTLS()
			defer ts.Close()
			// close hanging connection before shutting down the http server
			defer close(done)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			transport, ok := ts.Client().Transport.(*http.Transport)
			if !ok {
				t.Fatalf("failed to assert *http.Transport")
			}

			c, err := Config{
				Host:       ts.URL,
				Negotiator: newNegotiator(t),
			}.BuildManual(
				http_client.ManualCloneRequest(),
				EnableOption(!tt.enableHTTP2, http_client.DisableHttp2()),
				http_client.Timeout(1*time.Second),
				http_client.ManualDefaultClient(),
				http_client.ManualTransport(transport),
			)
			if err != nil {
				t.Fatalf("failed to create REST client: %v", err)
			}
			req1, err := c.Verb("GET").
				Prefix("foo").
				DoRaw(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			_, err = c.Verb("GET").
				Prefix("/hang").
				DoRaw(ctx)
			if err == nil {
				t.Fatalf("Expected error")
			}

			req2, err := c.Verb("GET").
				Prefix("foo").
				DoRaw(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// http1 doesn't reuse the connection after it times
			if tt.enableHTTP2 != (string(req1) == string(req2)) {
				if tt.enableHTTP2 {
					t.Fatalf("Expected %v to be the same as %v", string(req1), string(req2))
				} else {
					t.Fatalf("Expected %v to be different to %v", string(req1), string(req2))
				}
			}
		})
	}
}

func TestTransportConcurrency(t *testing.T) {
	const numReqs = 10
	var tests = []struct {
		name        string
		enableHTTP2 bool
	}{
		{"HTTP1", false},
		{"HTTP2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Logf("Connected from %v %v", r.RemoteAddr, r.URL)
				fmt.Fprintf(w, "%v", r.FormValue("echo"))
			}))
			ts.EnableHTTP2 = tt.enableHTTP2
			ts.StartTLS()
			defer ts.Close()
			var wg sync.WaitGroup

			wg.Add(numReqs)
			c := testRESTClient(t, ts)
			reqs := make(chan string)
			defer close(reqs)

			for i := 0; i < 4; i++ {
				go func() {
					for req := range reqs {
						res, err := c.Get().Param("echo", req).DoRaw(context.Background())
						if err != nil {
							t.Errorf("error on req %s: %v", req, err)
							wg.Done()
							continue
						}

						if string(res) != req {
							t.Errorf("body of req %s = %q; want %q", req, res, req)
						}

						wg.Done()
					}
				}()
			}
			for i := 0; i < numReqs; i++ {
				reqs <- fmt.Sprintf("request-%d", i)
			}
			wg.Wait()
		})
	}
}

// TODO: see if we can consolidate the other trackers into one.
type requestBodyTracker struct {
	io.ReadSeeker
	f func(string)
}

func (t *requestBodyTracker) Read(p []byte) (int, error) {
	t.f("Request.Body.Read")
	return t.ReadSeeker.Read(p)
}

func (t *requestBodyTracker) Seek(offset int64, whence int) (int64, error) {
	t.f("Request.Body.Seek")
	return t.ReadSeeker.Seek(offset, whence)
}

type responseBodyTracker struct {
	io.ReadCloser
	f func(string)
}

func (t *responseBodyTracker) Read(p []byte) (int, error) {
	t.f("Response.Body.Read")
	return t.ReadCloser.Read(p)
}

func (t *responseBodyTracker) Close() error {
	t.f("Response.Body.Close")
	return t.ReadCloser.Close()
}

type recorder struct {
	order []string
}

func (r *recorder) record(call string) {
	r.order = append(r.order, call)
}

func TestRequestBodyResetOrder(t *testing.T) {
	recorder := &recorder{}
	respBodyTracker := &responseBodyTracker{
		ReadCloser: nil, // the server will fill it
		f:          recorder.record,
	}

	var attempts int
	client := clientForFunc(func(req *http.Request) (*http.Response, error) {
		defer func() {
			attempts++
		}()

		// read the request body.
		ioutil.ReadAll(req.Body)

		// first attempt, we send a retry-after
		if attempts == 0 {
			resp := retryAfterResponse()
			respBodyTracker.ReadCloser = ioutil.NopCloser(bytes.NewReader([]byte{}))
			resp.Body = respBodyTracker
			return resp, nil
		}

		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	reqBodyTracker := &requestBodyTracker{
		ReadSeeker: bytes.NewReader([]byte{}), // empty body ensures one Read operation at most.
		f:          recorder.record,
	}
	req := &Request{
		verb: "POST",
		body: reqBodyTracker,
		c: &RESTClient{
			negotiator: newNegotiator(t),
			Client:     client,
		},
		backoff:    &noSleepBackOff{},
		maxRetries: 1,
		retryFn:    defaultRequestRetryFn,
	}

	req.Do(context.Background())

	expected := []string{
		// 1st attempt: the server handler reads the request body
		"Request.Body.Read",
		// the server sends a retry-after, client reads the
		// response body, and closes it
		"Response.Body.Read",
		"Response.Body.Close",
		// client retry logic seeks to the beginning of the request body
		"Request.Body.Seek",
		// 2nd attempt: the server reads the request body
		"Request.Body.Read",
	}
	if !reflect.DeepEqual(expected, recorder.order) {
		t.Errorf("Expected invocation request and response body operations for retry do not match: %s", cmp.Diff(expected, recorder.order))
	}
}
