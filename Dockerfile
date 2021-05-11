FROM golang:1.16 as builder
WORKDIR /w
COPY . /w
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /w/server /w

FROM scratch
COPY --from=builder /w/server server
ENTRYPOINT ["/server"]
