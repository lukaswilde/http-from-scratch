.PHONY: all image deploy-local clean

TAG ?= 1.0
CLUSTER_NAME ?= vmstack-cluster
VM_STACK_RELEASE ?= vmstack
VM_VERSION ?= 0.97.0
DOCKER_TAG ?= http-server:$(TAG)

all: deploy-local

image:
	docker build -t $(DOCKER_TAG) .

deploy-local: image
	@kind get clusters | grep -qx $(CLUSTER_NAME) || kind create cluster --config=k8s/kind-ingress.yaml --name $(CLUSTER_NAME)
	kind load docker-image $(DOCKER_TAG) --name $(CLUSTER_NAME)
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
	kubectl rollout status deployment/ingress-nginx-controller -n ingress-nginx --timeout=90s
	helm repo add victoriametrics https://victoriametrics.github.io/helm-charts/
	helm upgrade --install $(VM_STACK_RELEASE) victoriametrics/victoria-metrics-k8s-stack \
 				 --rollback-on-failure \
 				 --cleanup-on-fail \
 				 -f k8s/value-overrides.yaml \
 				 --version $(VM_VERSION)
	kubectl apply -f k8s/http-deployment.yaml \
				  -f k8s/http-service.yaml \
				  -f k8s/http-ingress.yaml
	kubectl rollout restart deployment/http-server-deployment

clean:
	kind delete cluster --name $(CLUSTER_NAME)
	-docker rmi $(DOCKER_TAG)