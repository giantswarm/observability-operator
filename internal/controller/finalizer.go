package controller

import (
	"context"
	"fmt"

	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type FinalizerHelper struct {
	// client is the client used to interact with the Kubernetes API
	client client.Client
	// finalizer is the finalizer string to be added/removed
	finalizer string
}

func NewFinalizerHelper(runtimeClient client.Client, finalizer string) FinalizerHelper {
	fh := FinalizerHelper{
		client:    runtimeClient,
		finalizer: finalizer,
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
	patchHelper, err := patch.NewHelper(object, fh.client)
	if err != nil {
		return false, fmt.Errorf("failed to create patch helper: %w", err)
	}

	// Add the finalizer to the object
	controllerutil.AddFinalizer(object, fh.finalizer)

	err = patchHelper.Patch(ctx, object)
	if err != nil {
		return false, fmt.Errorf("failed to patch object with finalizer %s: %w", fh.finalizer, err)
	}

	logger.Info("added finalizer", "finalizer", fh.finalizer)
	return true, nil
}

func (fh FinalizerHelper) EnsureRemoved(ctx context.Context, object client.Object) error {
	logger := log.FromContext(ctx)

	// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
	logger.Info("removing finalizer", "finalizer", fh.finalizer)
	patchHelper, err := patch.NewHelper(object, fh.client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper: %w", err)
	}

	// Remove the finalizer from the object
	controllerutil.RemoveFinalizer(object, fh.finalizer)

	err = patchHelper.Patch(ctx, object)
	if err != nil {
		return fmt.Errorf("failed to patch object to remove finalizer %s: %w", fh.finalizer, err)
	}

	logger.Info("removed finalizer", "finalizer", fh.finalizer)

	return nil
}
