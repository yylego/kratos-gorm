.PHONY: init
init:
	go install github.com/bufbuild/buf/cmd/buf@latest
	@echo "success!"

.PHONY: generate
generate:
	buf generate

.PHONY: clean
clean:
	rm -f errorspb/*.pb.go

.PHONY: help
help:
	@echo "init     - Install buf (protobuf code generation)"
	@echo "generate - regenerate errorspb PB from proto via buf"
	@echo "clean    - remove generated .pb.go files"
