.PHONY: build test frontend run clean

build: frontend
	go build -o bin/phonyc ./cmd/phonyc

frontend:
	cd web && npm install && npx vite build

test:
	go test ./internal/... -count=1

run: build
	PHONYC_ADDR=:8080 PHONYC_DATA_DIR=./data ./bin/phonyc

clean:
	rm -rf bin data web/node_modules
