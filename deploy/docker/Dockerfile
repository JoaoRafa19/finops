FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/finops ./cmd/finops

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=build /out/finops /app/finops
EXPOSE 10000
USER nonroot:nonroot
ENTRYPOINT ["/app/finops"]
