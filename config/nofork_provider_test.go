//go:build nofork

/*
 * Copyright (c) 2026 Oracle and/or its affiliates
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"testing"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestProviderConfigIncludesPreviouslySkippedNoForkResources(t *testing.T) {
	for name, provider := range map[string]func() *ujconfig.Provider{
		"cluster":    GetProvider,
		"namespaced": GetProviderNamespaced,
	} {
		t.Run(name, func(t *testing.T) {
			pc := provider()

			savedSearch := pc.Resources["oci_management_dashboard_management_saved_search"]
			if savedSearch == nil {
				t.Fatal("ManagementSavedSearch resource was not generated")
			}
			if !savedSearch.ShouldUseTerraformPluginSDKClient() {
				t.Fatal("ManagementSavedSearch is not routed through SDKv2 no-fork")
			}
			freeformTags := savedSearch.TerraformResource.Schema["freeform_tags"]
			if freeformTags == nil {
				t.Fatal("ManagementSavedSearch freeform_tags schema is missing")
			}
			freeformTagsElem, ok := freeformTags.Elem.(*schema.Schema)
			if !ok {
				t.Fatalf("ManagementSavedSearch freeform_tags Elem = %T, want *schema.Schema", freeformTags.Elem)
			}
			if freeformTagsElem.Type != schema.TypeString {
				t.Fatalf("ManagementSavedSearch freeform_tags Elem type = %s, want string", freeformTagsElem.Type)
			}

			opensearchCluster := pc.Resources["oci_opensearch_opensearch_cluster"]
			if opensearchCluster == nil {
				t.Fatal("OpensearchCluster resource was not generated")
			}
			if !opensearchCluster.ShouldUseTerraformPluginSDKClient() {
				t.Fatal("OpensearchCluster is not routed through SDKv2 no-fork")
			}
			samlConfig := opensearchCluster.TerraformResource.Schema["security_saml_config"]
			if samlConfig == nil {
				t.Fatal("OpensearchCluster security_saml_config schema is missing")
			}
			if samlConfig.Sensitive {
				t.Fatal("OpensearchCluster security_saml_config is sensitive, want only sensitive leaf fields")
			}
			samlResource, ok := samlConfig.Elem.(*schema.Resource)
			if !ok {
				t.Fatalf("OpensearchCluster security_saml_config Elem = %T, want *schema.Resource", samlConfig.Elem)
			}
			idpMetadataContent := samlResource.Schema["idp_metadata_content"]
			if idpMetadataContent == nil {
				t.Fatal("OpensearchCluster security_saml_config.idp_metadata_content schema is missing")
			}
			if !idpMetadataContent.Sensitive {
				t.Fatal("OpensearchCluster security_saml_config.idp_metadata_content is not sensitive")
			}

			zprPolicy := pc.Resources["oci_zpr_zpr_policy"]
			if zprPolicy == nil {
				t.Fatal("ZprPolicy resource was not generated")
			}
			for _, field := range []string{"defined_tags", "freeform_tags"} {
				strategy, ok := zprPolicy.ServerSideApplyMergeStrategies[field]
				if !ok {
					t.Fatalf("ZprPolicy %s SSA merge strategy is missing", field)
				}
				if strategy.MapMergeStrategy != ujconfig.MapTypeGranular {
					t.Fatalf("ZprPolicy %s map merge strategy = %q, want %q", field, strategy.MapMergeStrategy, ujconfig.MapTypeGranular)
				}
			}
		})
	}
}
