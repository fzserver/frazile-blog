# Pure-Go (modernc SQLite, embedded templates/assets), so the runtime image is
# scratch plus the CA bundle the Resend/ntfy clients need for outbound HTTPS.

FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go vet ./... && go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" \
      -o /out/blogd ./cmd/blogd

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/blogd /blogd

ENV ADDR=:8080 \
    DATA_DIR=/data \
    MEDIA_DIR=/data/media

EXPOSE 8080
USER 1000:1000
ENTRYPOINT ["/blogd"]
