.PHONY: build test run generate export docker-build docker-push clean

IMAGE_NAME ?= us-west1-docker.pkg.dev/farsipractice/farsipractice/falcon_demo_sf_mock
IMAGE_TAG  ?= latest

build:
	CGO_ENABLED=1 go build ./...

test:
	CGO_ENABLED=1 go test ./... -v -short

run:
	go run ./cmd/salesforce-mock/ -port 8080

generate:
	go run ./cmd/acme/ --profile profiles/acme_software.yaml --provider zai

export:
	go run ./cmd/export/

docker-build:
	docker build -f docker/Dockerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-push: docker-build
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

clean:
	rm -rf data/
