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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	gruntime "runtime"
	"strconv"
	"strings"
	"time"

	http_client "github.com/go418/http-client"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/pkg/version"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	// Environment variables: Note that the duration should be long enough that the backoff
	// persists for some reasonable time (i.e. 120 seconds).  The typical base might be "1".
	envBackoffBase     = "KUBE_CLIENT_BACKOFF_BASE"
	envBackoffDuration = "KUBE_CLIENT_BACKOFF_DURATION"
)

// Interface captures the set of operations for generically interacting with Kubernetes REST apis.
type Interface interface {
	Verb(verb string) *Request
	Post() *Request
	Put() *Request
	Patch(pt types.PatchType) *Request
	Get() *Request
	Delete() *Request
}

type Config struct {
	Host       string
	Negotiator SerializerNegotiator

	// Rate limiter for limiting connections to the master from this client. If present overwrites QPS/Burst
	RateLimiter flowcontrol.RateLimiter

	// QPS indicates the maximum QPS to the master from this client.
	// If it's zero, the created RESTClient will use DefaultQPS: 5
	QPS float32

	// Maximum burst for throttle.
	// If it's zero, the created RESTClient will use DefaultBurst: 10.
	Burst int

	// WarningHandler handles warnings in server responses.
	// If not set, the default warning handler is used.
	// See documentation for SetDefaultWarningHandler() for details.
	WarningHandler WarningHandler
}

const (
	DefaultQPS   float32 = 5.0
	DefaultBurst int     = 10
)

// TODO: move the default to secure when the apiserver supports TLS by default
// config.Insecure is taken to mean "I want HTTPS but don't bother checking the certs against a CA."
// hasCA := len(config.CAFile) != 0 || len(config.CAData) != 0
// hasCert := len(config.CertFile) != 0 || len(config.CertData) != 0
// defaultTLS := hasCA || hasCert || config.Insecure

func (o Config) Build(options ...http_client.Option) (*RESTClient, error) {
	options = append(options, http_client.UserAgent(DefaultKubernetesUserAgent()))
	client, err := http_client.NewClient(options...)
	if err != nil {
		return nil, err
	}
	if o.Host == "" {
		o.Host = "localhost"
	}
	base, err := DefaultServerURL(o.Host, true)
	if err != nil {
		return nil, err
	}

	rateLimiter := o.RateLimiter
	if rateLimiter == nil {
		qps := o.QPS
		if o.QPS == 0.0 {
			qps = DefaultQPS
		}
		burst := o.Burst
		if o.Burst == 0 {
			burst = DefaultBurst
		}
		if qps > 0 {
			rateLimiter = flowcontrol.NewTokenBucketRateLimiter(qps, burst)
		}
	}

	return NewRESTClient(base, o.Negotiator, readExpBackoffConfig, nil, o.WarningHandler, client)
}

func (o Config) BuildManual(options ...http_client.Option) (*RESTClient, error) {
	client, err := http_client.NewClientManual(options...)
	if err != nil {
		return nil, err
	}
	if o.Host == "" {
		o.Host = "localhost"
	}
	base, err := DefaultServerURL(o.Host, true)
	if err != nil {
		return nil, err
	}

	rateLimiter := o.RateLimiter
	if rateLimiter == nil {
		qps := o.QPS
		if o.QPS == 0.0 {
			qps = DefaultQPS
		}
		burst := o.Burst
		if o.Burst == 0 {
			burst = DefaultBurst
		}
		if qps > 0 {
			rateLimiter = flowcontrol.NewTokenBucketRateLimiter(qps, burst)
		}
	}

	return NewRESTClient(base, o.Negotiator, readExpBackoffConfig, rateLimiter, o.WarningHandler, client)
}

// readExpBackoffConfig handles the internal logic of determining what the
// backoff policy is.  By default if no information is available, NoBackoff.
// TODO Generalize this see #17727 .
func readExpBackoffConfig() BackoffManager {
	backoffBase := os.Getenv(envBackoffBase)
	backoffDuration := os.Getenv(envBackoffDuration)

	backoffBaseInt, errBase := strconv.ParseInt(backoffBase, 10, 64)
	backoffDurationInt, errDuration := strconv.ParseInt(backoffDuration, 10, 64)
	if errBase != nil || errDuration != nil {
		return &NoBackoff{}
	}
	return &URLBackoff{
		Backoff: flowcontrol.NewBackOff(
			time.Duration(backoffBaseInt)*time.Second,
			time.Duration(backoffDurationInt)*time.Second)}
}

// adjustCommit returns sufficient significant figures of the commit's git hash.
func adjustCommit(c string) string {
	if len(c) == 0 {
		return "unknown"
	}
	if len(c) > 7 {
		return c[:7]
	}
	return c
}

// adjustVersion strips "alpha", "beta", etc. from version in form
// major.minor.patch-[alpha|beta|etc].
func adjustVersion(v string) string {
	if len(v) == 0 {
		return "unknown"
	}
	seg := strings.SplitN(v, "-", 2)
	return seg[0]
}

// adjustCommand returns the last component of the
// OS-specific command path for use in User-Agent.
func adjustCommand(p string) string {
	// Unlikely, but better than returning "".
	if len(p) == 0 {
		return "unknown"
	}
	return filepath.Base(p)
}

// buildUserAgent builds a User-Agent string from given args.
func buildUserAgent(command, version, os, arch, commit string) string {
	return fmt.Sprintf(
		"%s/%s (%s/%s) kubernetes/%s", command, version, os, arch, commit)
}

// DefaultKubernetesUserAgent returns a User-Agent string built from static global vars.
func DefaultKubernetesUserAgent() string {
	return buildUserAgent(
		adjustCommand(os.Args[0]),
		adjustVersion(version.Get().GitVersion),
		gruntime.GOOS,
		gruntime.GOARCH,
		adjustCommit(version.Get().GitCommit))
}

func KubernetesUserAgent(userAgent string) http_client.Option {
	return http_client.UserAgent(DefaultKubernetesUserAgent() + "/" + userAgent)
}

func EnableOption(enable bool, option http_client.Option) http_client.Option {
	if !enable {
		return func(state *http_client.OptionState) error { return nil }
	}
	return option
}

// RESTClient imposes common Kubernetes API conventions on a set of resource paths.
// The baseURL is expected to point to an HTTP or HTTPS path that is the parent
// of one or more resources.  The server should return a decodable API resource
// object, or an api.Status object which contains information about the reason for
// any failure.
//
// Most consumers should use client.New() to get a Kubernetes API client.
type RESTClient struct {
	// base is the root URL for all invocations of the client
	base *url.URL

	// Negotiator is used for obtaining encoders and decoders for multiple
	// supported media types.
	negotiator SerializerNegotiator

	// creates BackoffManager that is passed to requests.
	createBackoffMgr func() BackoffManager

	// rateLimiter is shared among all requests created by this client unless specifically
	// overridden.
	rateLimiter flowcontrol.RateLimiter

	// warningHandler is shared among all requests created by this client.
	// If not set, defaultWarningHandler is used.
	warningHandler WarningHandler

	// Set specific behavior of the client.  If not set http.DefaultClient will be used.
	Client http_client.Client
}

// NewRESTClient creates a new RESTClient. This client performs generic REST functions
// such as Get, Put, Post, and Delete on specified paths.
func NewRESTClient(
	baseURL *url.URL,
	negotiator SerializerNegotiator,
	createBackoffMgr func() BackoffManager,
	rateLimiter flowcontrol.RateLimiter,
	warningHandler WarningHandler,
	client http_client.Client,
) (*RESTClient, error) {
	base := *baseURL
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	base.RawQuery = ""
	base.Fragment = ""

	return &RESTClient{
		base:             &base,
		negotiator:       negotiator,
		createBackoffMgr: createBackoffMgr,
		rateLimiter:      rateLimiter,
		warningHandler:   warningHandler,

		Client: client,
	}, nil
}

// Verb begins a request with a verb (GET, POST, PUT, DELETE).
//
// Example usage of RESTClient's request building interface:
// c, err := NewRESTClient(...)
// if err != nil { ... }
// resp, err := c.Verb("GET").
//  Path("pods").
//  SelectorParam("labels", "area=staging").
//  Timeout(10*time.Second).
//  Do()
// if err != nil { ... }
// list, ok := resp.(*api.PodList)
//
func (c *RESTClient) Verb(verb string) *Request {
	return NewRequest(c).Verb(verb)
}

// Post begins a POST request. Short for c.Verb("POST").
func (c *RESTClient) Post() *Request {
	return c.Verb("POST")
}

// Put begins a PUT request. Short for c.Verb("PUT").
func (c *RESTClient) Put() *Request {
	return c.Verb("PUT")
}

// Patch begins a PATCH request. Short for c.Verb("Patch").
func (c *RESTClient) Patch(pt types.PatchType) *Request {
	return c.Verb("PATCH").SetHeader("Content-Type", string(pt))
}

// Get begins a GET request. Short for c.Verb("GET").
func (c *RESTClient) Get() *Request {
	return c.Verb("GET")
}

// Delete begins a DELETE request. Short for c.Verb("DELETE").
func (c *RESTClient) Delete() *Request {
	return c.Verb("DELETE")
}
