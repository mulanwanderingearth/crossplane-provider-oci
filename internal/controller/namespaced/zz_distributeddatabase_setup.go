/*
Copyright 2022 Upbound Inc.
*/

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/v2/pkg/controller"

	distributedautonomousdatabase "github.com/oracle/provider-oci/internal/controller/namespaced/distributeddatabase/distributedautonomousdatabase"
	distributeddatabase "github.com/oracle/provider-oci/internal/controller/namespaced/distributeddatabase/distributeddatabase"
	distributeddatabaseprivateendpoint "github.com/oracle/provider-oci/internal/controller/namespaced/distributeddatabase/distributeddatabaseprivateendpoint"
)

// Setup_distributeddatabase creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup_distributeddatabase(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		distributedautonomousdatabase.Setup,
		distributeddatabase.Setup,
		distributeddatabaseprivateendpoint.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated_distributeddatabase creates all controllers with the supplied logger and adds them to
// the supplied manager gated.
func SetupGated_distributeddatabase(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		distributedautonomousdatabase.SetupGated,
		distributeddatabase.SetupGated,
		distributeddatabaseprivateendpoint.SetupGated,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
