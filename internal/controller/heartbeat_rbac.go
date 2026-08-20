package controller

// RBAC for the Heartbeat CRD. It lives here because RBAC generation only scans
// this package; the reconciler that uses these permissions follows in a separate
// change, and these markers move onto it then.

//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=heartbeats,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=heartbeats/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=heartbeats/finalizers,verbs=update
