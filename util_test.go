package kubeutil

import (
	"context"
	"testing"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type m = map[string]interface{}

func TestPatchOrCreate(t *testing.T) {

	ctx := context.Background()
	log := ctrl.Log.WithName("test")

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	var err error
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)

	c := fake.NewFakeClientWithScheme(scheme)

	nn := types.NamespacedName{Name: "test", Namespace: "test"}

	fetch := func() *v1.Deployment {
		obj := &v1.Deployment{}
		err = c.Get(ctx, nn, obj)
		if err != nil {
			t.Fatal(err)
		}
		return obj
	}

	u := unstructured.Unstructured{
		Object: m{
			"kind":       "Deployment",
			"apiVersion": "apps/v1",
			"metadata": m{
				"name":      nn.Name,
				"namespace": nn.Namespace,
			},
			"spec": m{
				"replicas": int64(1),
			},
		},
	}

	err = CreateOrUpdate(ctx, log, c, &u)
	if err != nil {
		t.Fatal(err)
	}

	// make sure get works without error
	_ = fetch()

	u.SetLabels(map[string]string{"applied": "by-patch"})
	err = CreateOrUpdate(ctx, log, c, &u)
	if err != nil {
		t.Fatal(err)
	}

	d := fetch()
	if d.GetLabels()["applied"] != "by-patch" {
		t.Fatal("expected the deployment to be patched with new labels")
	}

	// test updating with static object
	deployment := v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
			Labels: map[string]string{
				"applied": "static-object",
			},
		},
		Spec: v1.DeploymentSpec{},
	}
	err = CreateOrUpdate(ctx, log, c, &deployment)
	if err != nil {
		t.Fatal(err)
	}

	d = fetch()
	if d.GetLabels()["applied"] != "static-object" {
		t.Fatalf("expected the deployment to be patched with new labels: %s", d.GetLabels())
	}

	/*
		XXX does not currently work
		// test updating with mixed object
		um := unstructured.Unstructured{
			Object: m{
				"kind":       "Deployment",
				"apiVersion": "apps/v1",
				"metadata": m{
					"name":      nn.Name,
					"namespace": nn.Namespace,
					"labels": m{
						"applied": "mixed-object",
					},
				},
				"spec": &(v1.DeploymentSpec{}),
			},
		}
		err = CreateOrUpdate(ctx, log, c, &um)
		if err != nil {
			t.Fatal(err)
		}

		if d.GetLabels()["applied"] != "mixed-object" {
			t.Fatalf("expected the deployment to be patched with new labels: %s", d.GetLabels())
		}
	*/
}
