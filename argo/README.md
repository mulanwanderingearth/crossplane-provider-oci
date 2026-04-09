# Argo Workflows Guide

This directory contains the Argo assets used to run provider examples as Argo workflows against a Kubernetes cluster.

The directory is organized into four main areas:

- `argo/setup/`: bootstrap manifests for the workflow service account, token secret, PVC, and the default ProviderConfigs (cluster + namespaced).
- `argo/workflowtemplates/templates/`: hand-maintained reusable templates such as repository cloning, Crossplane installation, and generic create/delete resource helpers.
- `argo/workflowtemplates/generated-workflowtemplates/`: generated per-service `WorkflowTemplate` manifests.
- `argo/workflows/generated-workflows/`: generated top-level `Workflow` manifests that invoke one or more service templates.

Each service under `examples/<service>/<version>/` now contains `cluster/` and `namespaced/` subdirectories. The Argo generators emit distinct WorkflowTemplates for each scope so the consolidated workflow can execute cluster-scoped and namespaced tests independently.

Generated content is kept separate from hand-maintained templates so regeneration does not overwrite the shared workflow building blocks.

## Prerequisites

Before using these workflows, make sure you have:

- a working Kubernetes cluster and the correct `kubectl` context selected
- `kubectl`, `helm`, and the `argo` CLI installed locally
- permission to create namespaces, cluster roles, cluster role bindings, secrets, PVCs, and workflow resources
- OCI access that works with `InstancePrincipal`

Important: `argo/scripts/setup_crossplane.sh` creates the OCI credentials secret with `"auth": "InstancePrincipal"`, applies `argo/setup/cluster-providerconfig.yaml`, and—when `TEST_NAMESPACES` is set—applies `argo/setup/namespaced-providerconfig.yaml` into every requested namespace. If your test environment uses a different OCI auth method, update those files before running workflows.

## Bootstrap Argo Workflows

Install Argo Workflows into the `argo` namespace:

```bash
kubectl create namespace argo
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/download/v3.7.6/install.yaml
```

Create the service account, token secret, and PVC used by the workflows in this repo:

```bash
kubectl apply -f argo/setup/workflow-serviceaccount.yaml
kubectl apply -f argo/setup/workflow-admin-clusterrolebinding.yaml
kubectl apply -f argo/setup/workflow-token-secret.yaml
kubectl apply -f argo/setup/git-repo-pvc.yaml
```

These manifests use the `argo-workflow` service account in the `default` namespace because the checked-in workflow templates reference that identity directly.

To pre-provision Crossplane manually (for example, before the Argo job runs), execute:

```bash
TEST_NAMESPACES="team1,team2" \
TENANCY_OCID=<your-tenancy-ocid> \
REGION=us-ashburn-1 \
./argo/scripts/setup_crossplane.sh
```

The script installs Crossplane into `NAMESPACE` (defaults to `crossplane-system`), creates the `oci-creds` secret there, applies the cluster-scoped `ProviderConfig`, and then loops through `TEST_NAMESPACES` to create each namespace, seed its secret, and apply the namespaced `ProviderConfig`.

## Register the Shared Templates

Apply the reusable templates first:

```bash
kubectl apply -f argo/workflowtemplates/templates/
```

These three templates are the common runtime building blocks:

- `clone-repo-template`: clones this repository into the shared PVC mounted at `/git-repo`
- `crossplane-template`: installs or removes Crossplane and OCI providers
- `test-template`: creates, deletes, and describes individual resources from files under `examples/`

## Generate WorkflowTemplate and Workflow Manifests

The generated outputs in this repo come from two generators:

- `cmd/argo_workflowtemplate_generator`: creates per-service workflow templates
- `cmd/argo_workflow_generator`: creates the consolidated top-level workflow

Run them from the repository root:

```bash
go run ./cmd/argo_workflowtemplate_generator v1alpha1
go run ./cmd/argo_workflow_generator v1alpha1
```

`cmd/argo_workflowtemplate_generator` also accepts a repeatable `-service` flag if you only need to refresh specific templates (namespaced and cluster scopes are emitted for each service you list). For example, to regenerate just networking and compute:

```bash
go run ./cmd/argo_workflowtemplate_generator -service networking -service compute v1alpha1
```

If you only want to regenerate a subset of services in the top-level workflow, pass the service names after the version:

```bash
go run ./cmd/argo_workflow_generator v1alpha1 networking identity
```

After generation, apply the generated service templates:

```bash
kubectl apply -f argo/workflowtemplates/generated-workflowtemplates/cluster/
kubectl apply -f argo/workflowtemplates/generated-workflowtemplates/namespaced/
```

If you want to run the checked-in top-level workflows directly, they are currently:

```text
argo/workflows/generated-workflows/crossplane-provider-oci-cluster-v1alpha1.yaml
argo/workflows/generated-workflows/crossplane-provider-oci-namespaced-v1alpha1.yaml
```

## Optional: Access the Argo UI

Port-forward the Argo server:

```bash
kubectl -n argo port-forward deployment/argo-server 2746:2746
```

Fetch a bearer token for the `argo-workflow` service account:

```bash
ARGO_TOKEN="Bearer $(kubectl get secret argo-workflow.service-account-token -o=jsonpath='{.data.token}' | base64 --decode)"
echo "${ARGO_TOKEN}"
```

Then sign in at `https://localhost:2746`.

## Populate the Shared Repository PVC

Most workflows in this directory mount `git-repo-pvc` and expect the repository contents to exist at `/git-repo` inside the workflow pod. Populate that PVC before running service tests:

```bash
argo submit --from workflowtemplate/clone-repo-template \
  -p git_repo=https://github.com/crossplane-providers/crossplane-provider-oci.git \
  -p git_ref=main
```

If your Argo installation does not default ad hoc submissions to the `argo-workflow` service account, add:

```bash
--serviceaccount argo-workflow
```

## Install Crossplane and OCI Providers from Argo

Run the shared Crossplane setup template:

```bash
argo submit --from workflowtemplate/crossplane-template \
  --entrypoint setup-crossplane \
  -p namespace=crossplane-system \
  -p region=us-ashburn-1 \
  -p providers=provider-family-oci,provider-oci-networking \
  -p provider-image-repo-name=ghcr.io/oracle \
  -p family-provider-version=v1.0.1 \
  -p tenancy=<your-tenancy-ocid> \
  -p test-namespaces=team1,team2
```

This template:

- creates the target namespace if needed
- installs Crossplane with Helm
- installs the OCI family provider and any requested sub-providers
- creates the `oci-creds` secret in `crossplane-system`
- applies `argo/setup/cluster-providerconfig.yaml`
- applies `argo/setup/namespaced-providerconfig.yaml` into every namespace listed in `test-namespaces`

If you publish provider images to a private registry, change `provider-image-repo-name` and the version parameters accordingly.

## Run a Generated Top-Level Workflow

The checked-in generated workflows include the full set of service test tasks. The examples below show how to run only the networking service by enabling its scope-specific run flag.

Submit the cluster-scoped workflow like this:

```bash
argo submit argo/workflows/generated-workflows/crossplane-provider-oci-cluster-v1alpha1.yaml \
  -p run_networking_cluster_tests=true \
  -p availability_domain=<availability-domain> \
  -p compartment_ocid=<compartment-ocid> \
  -p image_instance_ocid=<image-source-instance-ocid> \
  -p create_compartment=true \
  -p create_image=true \
  -p create_instance=true \
  -p create_resources=true \
  -p delete_resources=true
```

Submit the namespaced workflow separately:

```bash
argo submit argo/workflows/generated-workflows/crossplane-provider-oci-namespaced-v1alpha1.yaml \
  -p run_networking_namespaced_tests=true \
  -p namespace_list='["team1","team2"]' \
  -p availability_domain=<availability-domain> \
  -p compartment_ocid=<compartment-ocid> \
  -p image_instance_ocid=<image-source-instance-ocid> \
  -p create_compartment=true \
  -p create_image=true \
  -p create_instance=true \
  -p create_resources=true \
  -p delete_resources=true
```

Parameter notes:

- `availability_domain`: used by the compute instance example referenced by the networking workflow
- `compartment_ocid`: parent or target compartment depending on whether the workflow creates a compartment
- `image_instance_ocid`: source instance OCID used by `examples/compute/v1alpha1/image.yaml`
- `delete_resources=true`: cleans up the created resources after validation
- `namespace_list`: JSON array of namespaces to exercise with namespaced templates. This applies to the namespaced workflow only. Use `[]` to skip namespaced runs or `["team1"]` / `["team1","team2"]` to run per namespace.

## Run a Generated Service WorkflowTemplate Directly

For faster reruns of a single service, submit the service template directly after applying it:

```bash
argo submit --from workflowtemplate/oci-networking-cluster-v1alpha1-tests-template \
  -p availability_domain=<availability-domain> \
  -p compartment_ocid=<compartment-ocid> \
  -p image_instance_ocid=<image-source-instance-ocid> \
  -p create_compartment=true \
  -p create_image=true \
  -p create_instance=true \
  -p create_resources=true \
  -p delete_resources=true
```

To run the namespaced variant, use `oci-networking-namespaced-v1alpha1-tests-template` and provide a namespace:

```bash
argo submit --from workflowtemplate/oci-networking-namespaced-v1alpha1-tests-template \
  -p target_namespace=team1 \
  -p availability_domain=<availability-domain> \
  -p compartment_ocid=<compartment-ocid> \
  -p image_instance_ocid=<image-source-instance-ocid> \
  -p create_compartment=true \
  -p create_image=true \
  -p create_instance=true \
  -p create_resources=true \
  -p delete_resources=true
```

The available service templates live under `argo/workflowtemplates/generated-workflowtemplates/` and follow this naming pattern:

```text
oci-<service>-<scope>-<version>-tests-template
```

## Troubleshooting

- If a workflow fails because `/git-repo/...` does not exist, rerun `clone-repo-template` to repopulate `git-repo-pvc`.
- If provider installation stalls, inspect `kubectl get providers` and the Crossplane pods in `crossplane-system`.
- If the UI token lookup fails, verify that `argo/setup/workflow-token-secret.yaml` exists in the `default` namespace and has been populated by the cluster.
- If OCI authentication fails, confirm that the cluster can use `InstancePrincipal`, or update the provider setup to use a different credential source.
