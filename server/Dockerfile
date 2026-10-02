FROM golang:1.22-alpine AS build
WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -mod=mod -o /juke-api .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /juke-api /juke-api
WORKDIR /
EXPOSE 8081
ENV PORT=8081
CMD ["/juke-api"]
