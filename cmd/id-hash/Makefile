id-hash:
	go build $(BUILD_OPTS) -o bin/id-hash ./cmd/id-hash

run-id-hash: id-hash
	./bin/id-hash --config=../configs/base.local.ini
