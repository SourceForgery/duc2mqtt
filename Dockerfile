FROM golang:1.22 AS builder

WORKDIR /app
ENV CGO_ENABLED=0

# Download deps
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o duc2mqtt ./src/

FROM scratch
COPY --from=builder /app/duc2mqtt /duc2mqtt
CMD [ "/duc2mqtt" ]
#ENTRYPOINT [ "/duc2mqtt" ]
