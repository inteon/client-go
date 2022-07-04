package rest

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type universalScheme struct {
	scheme *runtime.Scheme
}

func newUniversalScheme(scheme *runtime.Scheme) universalScheme {
	return universalScheme{
		scheme,
	}
}

func (t universalScheme) Name() string {
	return "UniversalScheme(" + t.scheme.Name() + ")"
}

var _ runtime.ObjectConvertor = universalScheme{}

func (s universalScheme) Convert(in, out interface{}, context interface{}) error {
	return s.scheme.Convert(in, out, context)
}

func (s universalScheme) ConvertFieldLabel(gvk schema.GroupVersionKind, label, value string) (string, string, error) {
	return s.scheme.ConvertFieldLabel(gvk, label, value)
}

func (s universalScheme) ConvertToVersion(in runtime.Object, target runtime.GroupVersioner) (runtime.Object, error) {
	return s.scheme.ConvertToVersion(in, target)
}

var _ runtime.ObjectCreater = universalScheme{}

func (c universalScheme) New(kind schema.GroupVersionKind) (runtime.Object, error) {
	obj, err := c.scheme.New(kind)
	c.scheme.Default(obj)
	if err != nil {
		ret := &unstructured.Unstructured{}
		ret.SetGroupVersionKind(kind)
		return ret, nil
	}
	return obj, nil
}

var _ runtime.ObjectTyper = universalScheme{}

func (t universalScheme) Recognizes(gvk schema.GroupVersionKind) bool {
	return true
}

func (t universalScheme) ObjectKinds(obj runtime.Object) ([]schema.GroupVersionKind, bool, error) {
	gvks, unversioned, err := t.scheme.ObjectKinds(obj)
	if err == nil {
		return gvks, unversioned, err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if len(gvk.Kind) == 0 {
		return nil, false, runtime.NewMissingKindErr("object has no Kind field")
	}
	if len(gvk.Version) == 0 {
		return nil, false, runtime.NewMissingVersionErr("object has no ApiVersion field")
	}

	return []schema.GroupVersionKind{gvk}, false, nil
}

var _ runtime.ObjectDefaulter = universalScheme{}

func (t universalScheme) Default(in runtime.Object) {
	t.scheme.Default(in)
}
