package controller

import (
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type FinalizerHelper struct {
	// runtimeClient is the client used to interact with the Kubernetes API
	runtimeClient client.Client
	// finalizer is the finalizer string to be added/removed
	finalizer string
}

func NewFinalizerHelper(runtimeClient client.Client, finalizer string) FinalizerHelper {
	fh := FinalizerHelper{
		runtimeClient: runtimeClient,
		finalizer:     finalizer,
	}

	return fh
}

func (fh FinalizerHelper) EnsureAdded(ctx context.Context, object client.Object) (bool, error) {
	if controllerutil.ContainsFinalizer(object, fh.finalizer) {
		// Finalizer already exists, no need to add it
		return false, nil
	}

	logger := log.FromContext(ctx)

	// We use a patch rather than an update to avoid conflicts when multiple controllers are adding their finalizer
	// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
	logger.Info("adding finalizer", "finalizer", fh.finalizer)
	patchHelper, err := patch.NewHelper(object, fh.runtimeClient)
	if err != nil {
		return false, errors.WithStack(err)
	}

	// Add the finalizer to the object
	controllerutil.AddFinalizer(object, fh.finalizer)

	err = patchHelper.Patch(ctx, object)
	if err != nil {
		logger.Error(err, "failed to add finalizer", "finalizer", fh.finalizer)
		return false, errors.WithStack(err)
	}

	logger.Info("added finalizer", "finalizer", fh.finalizer)
	return true, nil
}

func (fh FinalizerHelper) EnsureRemoved(ctx context.Context, object client.Object) error {
	logger := log.FromContext(ctx)

	// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
	logger.Info("removing finalizer", "finalizer", fh.finalizer)
	patchHelper, err := patch.NewHelper(object, fh.runtimeClient)
	if err != nil {
		return errors.WithStack(err)
	}

	// Remove the finalizer from the object
	controllerutil.RemoveFinalizer(object, fh.finalizer)

	err = patchHelper.Patch(ctx, object)
	if err != nil {
		logger.Error(err, "failed to remove finalizer", "finalizer", fh.finalizer)
		return errors.WithStack(err)
	}

	logger.Info("removed finalizer", "finalizer", fh.finalizer)

	return nil
}
