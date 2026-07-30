.PHONY: build test frontend run clean

build: frontend
	go build -o bin/phonyg ./cmd/phonyg

frontend:
	cd web && npm install && npx vite build

test:
	go test ./internal/... -count=1

run: build
	PHONYG_ADDR=0.0.0.0:23342 PHONYG_DATA_DIR=./data ./bin/phonyg

clean:
	rm -rf bin data web/node_modules

.PHONY: deploy
deploy:
	./scripts/deploy.sh
