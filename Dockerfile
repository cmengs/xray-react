# Build stage
FROM golang:1.20-alpine AS build
WORKDIR /src
COPY . .
RUN apk add --no-cache gcc musl-dev
RUN go build -o /collector ./cmd/collector

# Run stage
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /collector /usr/local/bin/collector
EXPOSE 8080
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/collector"]
