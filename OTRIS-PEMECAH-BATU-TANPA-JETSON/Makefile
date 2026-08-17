.PHONY: tidy run-capture-file run-capture-passive help

tidy:
	go mod tidy

build:
	go build -o bin/pemecah-batu ./cmd/pemecah-batu

run-capture-file: build
	@echo "MSH|^~\\&|ABC|001|70|bpm" > /tmp/sample_hl7.txt
	./bin/pemecah-batu capture --mode file --file /tmp/sample_hl7.txt --out captures/test_file --duration 2s
	cat captures/test_file/manifest.json

run-capture-passive: build
	./bin/pemecah-batu capture --mode passive --listen 0.0.0.0:4000 --out captures/passive_$(shell date +%s) --duration 30s

help: build
	./bin/pemecah-batu --help
	./bin/pemecah-batu capture --help
