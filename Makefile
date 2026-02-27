BINARY := bundle-broadcaster
GO := go
GOFLAGS := -v

.PHONY: build install clean

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) .

install: build
	$(GO) install .

clean:
	rm -f $(BINARY)
