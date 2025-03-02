FEED_API_BIN=../build/feed-api
FEED_CURATOR_BIN=../build/feed-curator
ACTIONS_BIN=../build/actions

FEED_API_DIR=feed-api
FEED_CURATOR_DIR=feed-curator
ACTIONS_DIR=actions

GO_VERSION=1.23

all: build

build: build-feed-api build-feed-curator build-actions

build-feed-api:
	@echo "Building feed-api..."
	@cd $(FEED_API_DIR) && go build -o $(FEED_API_BIN) .

build-feed-curator:
	@echo "Building feed-curator..."
	@cd $(FEED_CURATOR_DIR) && go build -o $(FEED_CURATOR_BIN) .

build-actions:
	@echo "Building actions..."
	@cd $(ACTIONS_DIR) && go build -o $(ACTIONS_BIN) .

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(FEED_API_DIR)/$(FEED_API_BIN) $(FEED_CURATOR_DIR)/$(FEED_CURATOR_BIN) $(ACTIONS_DIR)/$(ACTIONS_BIN)

fmt:
	@echo "Formatting Go code..."
	@go fmt ./...

tidy:
	@echo "Tidying Go modules..."
	@cd $(FEED_API_DIR) && go mod tidy
	@cd $(FEED_CURATOR_DIR) && go mod tidy
	@cd $(ACTIONS_DIR) && go mod tidy

deps:
	@echo "Installing Go dependencies..."
	@go mod download

.PHONY: all build build-feed-api build-feed-curator build-actions clean fmt tidy deps
