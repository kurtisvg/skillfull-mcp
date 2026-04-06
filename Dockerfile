# Build stage
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -o /bin/skillful-mcp .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/skillful-mcp /bin/skillful-mcp
COPY --from=build /src/LICENSE /LICENSE
ENTRYPOINT ["/bin/skillful-mcp"]
