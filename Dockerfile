FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker && CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate && CGO_ENABLED=0 go build -o /out/relay ./cmd/relay && CGO_ENABLED=0 go build -o /out/insights ./cmd/insights && CGO_ENABLED=0 go build -o /out/recovery ./cmd/recovery
FROM alpine:3.21
ARG FOODFLOW_SOURCE_DIGEST=unbound
LABEL io.foodflow.source-digest=$FOODFLOW_SOURCE_DIGEST
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/ /app/
COPY sql/schema /app/sql/schema
EXPOSE 8080
CMD ["/app/api"]
