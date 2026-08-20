package pointintimerestore

import (
	"context"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func (r *Reconciler) restorePrimaryStorage(_ context.Context, _ *v1.PointInTimeRestore) (hold, error) {
	return settle, nil
}
