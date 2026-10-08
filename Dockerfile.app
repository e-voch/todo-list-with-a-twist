FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN ls -la
RUN go mod download
COPY . .
RUN go build -o todo ./cmd/todo

FROM gcr.io/distroless/base-debian13:nonroot
WORKDIR /app
COPY --from=build /src/todo .
EXPOSE 8080
ENTRYPOINT ["./todo"]