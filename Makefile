PROTO_DIR := proto
GO_OUT_DIR := api
GO_SERVER_DIR := $(GO_OUT_DIR)/serv/v1
BIN_NAME := vk-server
MAIN_PKG := ./cmd/server
PKGS := ./...

PROTOC := protoc
PROTOC_GEN_GO := $(shell which protoc-gen-go)
PROTOC_GEN_GRPC := $(shell which protoc-gen-go-grpc)

proto: $(PROTO_DIR)/pubsub.proto
	@echo "Generating Go code from proto files..."
	@$(PROTOC) \
		-I $(PROTO_DIR) \
		--go_out=$(GO_OUT_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/pubsub.proto
	@rm -rf $(GO_SERVER_DIR)
	@mkdir -p $(GO_SERVER_DIR)
	@mv $(GO_OUT_DIR)/pubsub* $(GO_SERVER_DIR)
	@echo "Go code generated successfully."
.PHONY: proto

test:
	go test -race $(PKGS)
.PHONY: test

build:
	go build -o $(BIN_NAME) $(MAIN_PKG)
.PHONY: build

run: proto
	go run $(MAIN_PKG)
.PHONY: run

docker: proto build
	@echo "Building Docker image..."
	docker build -t vk-server:latest .
	@echo "Docker image built successfully."
.PHONY: docker

clean:
	rm -rf $(BIN_NAME)
.PHONY: clean