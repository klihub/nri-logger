GO_PARALLEL :=
GO_CMD      := go
GO_BUILD    := $(GO_CMD) build $(GO_PARALLEL)
GO_FMT      := gofmt
GO_LINT     := golint
GO_CILINT   := golangci-lint

BINARIES := logger-v1 logger-vproto

all: $(BINARIES)

logger-v1: v1/nri-logger.go
	$(GO_BUILD) $(BUILD_TAGS) $(LDFLAGS) $(GCFLAGS) -o $@ ./v1

logger-vproto: vproto/nri-logger.go
	$(GO_BUILD) $(BUILD_TAGS) $(LDFLAGS) $(GCFLAGS) -o $@ ./vproto

clean:
	rm -f $(BINARIES)
