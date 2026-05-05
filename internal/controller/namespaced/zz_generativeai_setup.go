/*
Copyright 2022 Upbound Inc.
*/

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/v2/pkg/controller"

	agentagent "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/agentagent"
	agentagentendpoint "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/agentagentendpoint"
	agentdataingestionjob "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/agentdataingestionjob"
	agentdatasource "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/agentdatasource"
	agentknowledgebase "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/agentknowledgebase"
	agentprovisionedcapacity "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/agentprovisionedcapacity"
	agenttool "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/agenttool"
	dedicatedaicluster "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/dedicatedaicluster"
	endpoint "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/endpoint"
	generativeaiprivateendpoint "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/generativeaiprivateendpoint"
	importedmodel "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/importedmodel"
	model "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/model"
	semanticstore "github.com/oracle/provider-oci/internal/controller/namespaced/generativeai/semanticstore"
)

// Setup_generativeai creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup_generativeai(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		agentagent.Setup,
		agentagentendpoint.Setup,
		agentdataingestionjob.Setup,
		agentdatasource.Setup,
		agentknowledgebase.Setup,
		agentprovisionedcapacity.Setup,
		agenttool.Setup,
		dedicatedaicluster.Setup,
		endpoint.Setup,
		generativeaiprivateendpoint.Setup,
		importedmodel.Setup,
		model.Setup,
		semanticstore.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated_generativeai creates all controllers with the supplied logger and adds them to
// the supplied manager gated.
func SetupGated_generativeai(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		agentagent.SetupGated,
		agentagentendpoint.SetupGated,
		agentdataingestionjob.SetupGated,
		agentdatasource.SetupGated,
		agentknowledgebase.SetupGated,
		agentprovisionedcapacity.SetupGated,
		agenttool.SetupGated,
		dedicatedaicluster.SetupGated,
		endpoint.SetupGated,
		generativeaiprivateendpoint.SetupGated,
		importedmodel.SetupGated,
		model.SetupGated,
		semanticstore.SetupGated,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
