GO = $(shell git rev-parse --show-toplevel)/.setup/.go/go/bin/go
GOFLAGS = -mod=vendor
APPBIN  = bin/emissary

all: $(GO)
	$(GO) build $(GOFLAGS) -o $(APPBIN) ./cmd/emissary

fmt: $(GO)
	$(GO) fmt $(shell $(GO) list ./.../)

gen:
	mockery -name=HealthCheck --dir=pkg/spire/ --output=mocks/ --outpkg mocks
	mockery -name=JWTSVID --dir=pkg/spire/ --output=mocks/ --outpkg mocks
	mockery -name=SpiffeWorkloadAPIClient --dir=vendor/github.com/spiffe/go-spiffe/proto/spiffe/workload/ --output=mocks/ --outpkg mocks

docker:
	docker build .

test: $(GO)
	mkdir -p tmp
	$(GO) test $(GOFLAGS) -race -cover -coverprofile=tmp/coverage.out ./...

$(GO):
	script/install-go
