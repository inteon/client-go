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

package fake

import (
	"k8s.io/client-go/discovery"

	fakerawdiscovery "k8s.io/client-go/discovery/raw/fake"
)

// FakeDiscovery implements discovery.DiscoveryInterface and sometimes calls testing.Fake.Invoke with an action,
// but doesn't respect the return value if any. There is a way to fake static values like ServerVersion by using the Faked... fields on the struct.
type FakeDiscovery struct {
	discovery.DiscoveryInterface

	FakeRawDiscoveryClient fakerawdiscovery.FakeRawDiscoveryClient
}

func NewFakeDiscovery() *FakeDiscovery {
	fake := &FakeDiscovery{}
	fake.DiscoveryInterface = discovery.NewDiscoveryClientFromRaw(&fake.FakeRawDiscoveryClient)
	return fake
}
