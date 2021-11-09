.PHONY: terraform/plan
terraform/plan:
	cd terraform && \
	terraform init -upgrade -backend-config="bucket=${TFSTATE_BUCKET}" && \
	terraform plan

.PHONY: terraform/apply
terraform/apply:
	cd terraform && \
	terraform apply

bootstrap:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -o bootstrap .

.PHONY: deploy
deploy: bootstrap
	lambroll --tfstate terraform/.terraform/terraform.tfstate deploy

.PHONY: deploy-dry-run
deploy-dry-run: bootstrap
	lambroll --tfstate terraform/.terraform/terraform.tfstate deploy --dry-run
