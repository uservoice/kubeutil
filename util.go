package kubeutil

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// IgnoreNotFound returns nil when the err is kubernetes NotFound, otherwise
// returns the original err
func IgnoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

// mutateFn is used to reconsicle differences when the object is getting
// updated. src is the existing object while dest is what the object will be
// updated to.
type mutateFn func(src, dest runtime.Object)

// CreateOrUpdate
func CreateOrUpdate(ctx context.Context, log logr.Logger, c client.Client, obj runtime.Object) error {

	nn, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}

	fns := []mutateFn{}

	switch obj.(type) {
	case *corev1.Service:
		fns = append(fns, func(src, dest runtime.Object) {
			// clusterIp cannot be updated on a service, so copy the field
			dest.(*corev1.Service).Spec.ClusterIP = src.(*corev1.Service).Spec.ClusterIP
		})
	}

	res, err := createOrUpdate(ctx, c, obj, fns...)
	log.V(1).Info(string(res), "name", nn.String(), "object", reflect.TypeOf(obj))
	return err
}

// for reference https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/controller/controllerutil/controllerutil.go#L136
func createOrUpdate(ctx context.Context, c client.Client, obj runtime.Object, mutateFn ...mutateFn) (controllerutil.OperationResult, error) {

	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	existing := obj.DeepCopyObject()
	if err := c.Get(ctx, key, existing); err != nil {
		if !errors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		if err := c.Create(ctx, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, nil
	}

	if err := setMatchingResourceVersion(existing, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}

	for _, fn := range mutateFn {
		fn(existing, obj)
	}

	if reflect.DeepEqual(existing, obj) {
		return controllerutil.OperationResultNone, err
	}

	if err := c.Update(ctx, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}
	return controllerutil.OperationResultUpdated, nil
}

// setMatchingResourceVersion sets the ResourceVersion to be the same as the
// first argument
func setMatchingResourceVersion(from, to runtime.Object) error {

	src, err := meta.Accessor(from)
	if err != nil {
		return err
	}

	dest, err := meta.Accessor(to)
	if err != nil {
		return err
	}

	dest.SetResourceVersion(src.GetResourceVersion())
	return nil
}
