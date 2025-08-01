gpsi:
	go build $(BUILD_OPTS) -o bin/gpsi ./cmd/gpsi

run-gpsi: gpsi
	./bin/gpsi --config=../configs/base.local.ini
