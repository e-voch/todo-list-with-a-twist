# One image for all three programs. The default entrypoint runs the web
# server; override it with ./worker or ./migrate to run the others.
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/todo ./cmd/todo && \
    go build -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/base-debian13:nonroot
WORKDIR /app
COPY --from=build /out/ .
EXPOSE 8080
ENTRYPOINT ["./todo"]
