
FROM golang:1.25-alpine AS builder

WORKDIR /app


COPY go.mod go.sum ./
RUN go mod download


COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/velocity-server ./cmd/server



FROM gcr.io/distroless/static-debian11


COPY --from=builder /bin/velocity-server /velocity-server


EXPOSE 8080


CMD ["/velocity-server"]