UNAME := $(shell uname)

ifeq ($(UNAME), Linux)
target := linux
endif
ifeq ($(UNAME), Darwin)
target := darwin
endif

build:
	GOOS=$(target) go build -o "bin/gen" ./cmd/$*

currentValues := tmp
destination := generated
resources := resources

generate:
	echo $${ENCODED_SPIN_CONFIG} | base64 -d > ~/.spin/config
	aws s3 sync s3://$${AWS_S3_BUCKET}/values $(currentValues)

	gen --destination $(destination) --values $(resources)

	# Upload values to S3
	aws s3 sync values s3://$${AWS_S3_BUCKET}/values

	# Deploy spinnaker resources
	ls -1 /$(destination)/applications | xargs -I{} spin app save --file /$(destination)/applications/{}
	ls -1 /$(destination)/pipelines | xargs -I{} spin pi save --file /$(destination)/pipelines/{}