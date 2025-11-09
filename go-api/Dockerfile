FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /main .

FROM scratch
COPY --from=builder /main /main

EXPOSE 8080

CMD ["/main"]