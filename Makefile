TFSTATE_PATH:=terraform/.terraform/terraform.tfstate

.PHONY: terraform/plan
terraform/plan:
	cd terraform && \
	terraform init -upgrade -backend-config="bucket=${TFSTATE_BUCKET}" && \
	terraform plan

.PHONY: terraform/apply
terraform/apply:
	cd terraform && \
	terraform apply

.PHONY: terraform/apply-only-iam
terraform/apply-only-iam:
	cd terraform && \
	terraform apply -target={aws_iam_role.function,aws_iam_role_policy.function,aws_s3_bucket.data}

bootstrap:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -o bootstrap .

.PHONY: deploy
deploy: bootstrap
	lambroll --tfstate $(TFSTATE_PATH) deploy

.PHONY: deploy-dry-run
deploy-dry-run: bootstrap
	lambroll --tfstate $(TFSTATE_PATH) deploy --dry-run

.PHONY: reset-counter
reset-counter:
	aws s3 rm s3://${S3_BUCKET}/${S3_OBJECT_PATH}

total_access:=500
parallelism:=20
.PHONY: reset-counter
load-test:
	ab -n ${total_access} -c ${parallelism} $(shell tfstate-lookup -state ${TFSTATE_PATH} aws_apigatewayv2_api.main.api_endpoint)/
