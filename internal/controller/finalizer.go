package controller

import (
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func ensureFinalizerdAdded(ctx context.Context, runtimeClient client.Client, object client.Object, finalizer string) (bool, error) {
	if controllerutil.ContainsFinalizer(object, finalizer) {
		// Finalizer already exists, no need to add it
		return false, nil
	}

	logger := log.FromContext(ctx)

	// We use a patch rather than an update to avoid conflicts when multiple controllers are adding their finalizer
	// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
	logger.Info("adding finalizer", "finalizer", finalizer)
	patchHelper, err := patch.NewHelper(object, runtimeClient)
	if err != nil {
		return false, errors.WithStack(err)
	}

	// Add the finalizer to the object
	controllerutil.AddFinalizer(object, finalizer)

	err = patchHelper.Patch(ctx, object)
	if err != nil {
		logger.Error(err, "failed to add finalizer", "finalizer", finalizer)
		return false, errors.WithStack(err)
	}

	logger.Info("added finalizer", "finalizer", finalizer)
	return true, nil
}

func ensureFinalizerRemoved(ctx context.Context, runtimeClient client.Client, object client.Object, finalizer string) error {
	logger := log.FromContext(ctx)

	// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
	logger.Info("removing finalizer", "finalizer", finalizer)
	patchHelper, err := patch.NewHelper(object, runtimeClient)
	if err != nil {
		return errors.WithStack(err)
	}

	// Remove the finalizer from the object
	controllerutil.RemoveFinalizer(object, finalizer)

	err = patchHelper.Patch(ctx, object)
	if err != nil {
		logger.Error(err, "failed to remove finalizer", "finalizer", finalizer)
		return errors.WithStack(err)
	}

	logger.Info("removed finalizer", "finalizer", finalizer)

	return nil
}
