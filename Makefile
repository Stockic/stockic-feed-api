FEED_API_BIN=../build/feed-api
FEED_CURATOR_BIN=../build/feed-curator

FEED_API_DIR=feed-api
FEED_CURATOR_DIR=feed-curator

GO_VERSION=1.23

all: build

build: build-feed-api build-feed-curator

build-feed-api:
	@echo "Building feed-api..."
	@cd $(FEED_API_DIR) && go build -o $(FEED_API_BIN) .

build-feed-curator:
	@echo "Building feed-curator..."
	@cd $(FEED_CURATOR_DIR) && go build -o $(FEED_CURATOR_BIN) .

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(FEED_API_DIR)/$(FEED_API_BIN) $(FEED_CURATOR_DIR)/$(FEED_CURATOR_BIN)

fmt:
	@echo "Formatting Go code..."
	@go fmt ./...

tidy:
	@echo "Tidying Go modules..."
	@cd $(FEED_API_DIR) && go mod tidy
	@cd $(FEED_CURATOR_DIR) && go mod tidy

deps:
	@echo "Installing Go dependencies..."
	@go mod download

.PHONY: all build build-feed-api build-feed-curator test-feed-api test-feed-curator clean fmt tidy deps
