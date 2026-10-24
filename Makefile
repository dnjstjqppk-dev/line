.PHONY: test-go test-smoke dev-up dev-down user-build

test-go:
	cd backend && go test ./...
test-smoke:
	python3 scripts/smoke_test.py
dev-up:
	docker compose up --build -d
dev-down:
	docker compose down
user-build:
	npm run build -w @messenger/user-web
