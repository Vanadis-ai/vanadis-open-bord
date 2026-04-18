FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /amail .
RUN CGO_ENABLED=0 go build -o /amail-seed ./cmd/seed

FROM gcr.io/distroless/static-debian12
COPY --from=builder /amail /amail
COPY --from=builder /amail-seed /amail-seed
EXPOSE 8080
ENTRYPOINT ["/amail"]
