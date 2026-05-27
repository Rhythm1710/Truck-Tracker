# --- build stage ---
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server -ldflags="-s -w" ./cmd/server

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
COPY migrations /migrations
EXPOSE 8080 5023
ENTRYPOINT ["/server"]
