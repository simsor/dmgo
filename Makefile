ZIP=zip

GOOS=linux
GOARCH=arm
GOARM=7

GO=go

.PHONY: dmgo

all: dmgo

dmgo:
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(GO) build ./cmd/dmgo

extension: dmgo
	mv dmgo extensions/dmgo/
	$(ZIP) -r kindle_gameboy.zip extensions