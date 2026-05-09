FROM golang:1.25 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/rigr ./cmd/rigr

FROM gcr.io/distroless/static:nonroot

ENV HTTP_BIND=0.0.0.0:8080
EXPOSE 8080

COPY --from=build /out/rigr /rigr
USER nonroot:nonroot
ENTRYPOINT ["/rigr"]

