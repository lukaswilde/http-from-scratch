.PHONY: all image deploy-local clean

SERVER_VERSION ?= 1.0
CLUSTER_NAME ?= vmstack
DOCKER_TAG ?= http-server:$(SERVER_VERSION)

all: deploy-local

image:
	docker build -t $(DOCKER_TAG) .

deploy-local: image
	@kind get clusters | grep -qx $(CLUSTER_NAME) || kind create cluster --config=k8s/kind-ingress.yaml --name $(CLUSTER_NAME)
	kind load docker-image $(DOCKER_TAG) --name $(CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
	kubectl rollout status deployment/ingress-nginx-controller -n ingress-nginx --timeout=90s
	kubectl apply -f k8s/http-deployment.yaml \
				  -f k8s/http-service.yaml \
				  -f k8s/http-ingress.yaml

clean:
	kind delete cluster --name $(CLUSTER_NAME)
	-docker rmi $(DOCKER_TAG)