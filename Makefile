run:
	cd app && go run ./cmd/chat-logger

test:
	cd app && go test ./...

lint:
	@echo "Lint target not configured. Add a linter and update this target."
