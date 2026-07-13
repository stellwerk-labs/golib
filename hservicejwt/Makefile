# Disable all the default make stuff
MAKEFLAGS += --no-builtin-rules
.SUFFIXES:

.PHONY: test
test:
	go test ./... -cover -race
