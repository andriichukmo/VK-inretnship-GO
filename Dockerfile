FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o vk-server ./cmd/server

FROM gcr.io/distroless/static:nonroot
WORKDIR /home/nonroot
COPY --from=builder /app/vk-server .
COPY configs/config.yaml ./configs/

EXPOSE 50051
USER nonroot:nonroot
ENTRYPOINT [ "./vk-server" ]