
LOCAL_BIN := $(CURDIR)/bin
bin-deps:
	GOBIN=$(LOCAL_BIN) go install github.com/vektra/mockery/v3@latest

generate-mocks:
	$(LOCAL_BIN)/mockery --config .mockery.yaml
