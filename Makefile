PLATFORMS := linux/amd64 windows/amd64 linux/arm darwin/amd64
temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))
GIT_SHA=$(shell git rev-parse HEAD)
SOURCES==$(wildcard *.go)

release: version.go
	make -l inner_release

.PHONY: inner_release
inner_release: $(PLATFORMS)

version.go: main.go
	head -n -1 version.go > /tmp/version.go; mv /tmp/version.go version.go
	echo 'const currentVersion = "$(GIT_SHA)"' >> version.go

$(PLATFORMS):
	@echo "Building for $(os)-$(arch)"
	@-mkdir -p dist/$(os)-$(arch)
	@-rm -r dist/$(os)-$(arch)
	GOOS=$(os) GOARCH=$(arch) go build -o 'dist/sonic_$(os)_$(arch)' .
	@chmod +x dist/sonic_$(os)_$(arch)
	@if [ $(os) = windows ]; then mv dist/sonic_$(os)_$(arch) dist/sonic_$(os)_$(arch).exe; fi

test:
	bash .envrc && go test
