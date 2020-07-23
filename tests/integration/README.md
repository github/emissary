### Emissary Integration Tests

This uses docker compose to setup the following containers:
* spire-server
* envoy
* httpbin based app container
* spire-agent+emissary (in a single container)

Once up and running the test suite in `tests` makes numerous HTTP requests via `curl` to all of these containers to ensure proper behavior.

You can run the integration test suite using `./script/cibuild-emissary-integration`.
