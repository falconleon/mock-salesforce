FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o /salesforce-mock ./cmd/salesforce-mock/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget tzdata && \
    adduser -D -u 1000 appuser

COPY --from=builder /salesforce-mock /usr/local/bin/salesforce-mock
COPY testdata/ /app/testdata/

USER appuser
WORKDIR /app
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["salesforce-mock"]
CMD ["-port", "8080", "-seed", "/app/testdata/seed"]
