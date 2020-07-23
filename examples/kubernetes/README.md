### Kubernetes Deployment Example

This is an example of what a kubernetes deployment of emissary might look like. It includes the application deployment along with envoy and emissary sidecar containers.

A few important details:
* This configuration expects spire-agent socket is available on each kube node as `/run/spire/sockets/agent.sock`
* Envoy listens on port 443 as the exposed port for the pod
* This deployment deploys a service called `very-dope-application`
* `cool-service` is the identifier for the service you expect `very-dope-application` to accept requests from (ingress)
* `nice-service` is the identifier for the service you expect `very-dope-application` to make requests to (egress)
* Requests to `nice-service` are made from the `very-dope-application` pod to Envoy for egress using `localhost:18000` where Envoy proxies the request to an ingress controller, load balancer or other endpoint
* Envoy `ext_authz` filtering is only enabled for requests containing the `x-emissary-auth` header, i.e. only requests originating from other pods with emissary enabled
