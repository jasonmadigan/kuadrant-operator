
##@ Kuadrant controller manifests

.PHONY: kuadrant-controller-manifests-template
kuadrant-controller-manifests-template: export KUADRANT_CONTROLLER_GITREF := $(KUADRANT_CONTROLLER_GITREF)
kuadrant-controller-manifests-template:
	envsubst \
        < $(PROJECT_PATH)/config/kuadrant-controller-manifests/kustomization.template.yaml \
        > $(PROJECT_PATH)/config/kuadrant-controller-manifests/kustomization.yaml

.PHONY: kuadrant-controller-manifests
kuadrant-controller-manifests: kuadrant-controller-manifests-template kustomize ## Update kuadrant controller manifests.
	$(KUSTOMIZE) build config/kuadrant-controller-manifests -o $(PROJECT_PATH)/kuadrantcontrollermanifests/autogenerated/kuadrant-controller.yaml
