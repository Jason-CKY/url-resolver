FROM golang:1.21-alpine as build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -o /url-resolver

FROM scratch as production

WORKDIR /

COPY --from=build /url-resolver /url-resolver

EXPOSE 8080
CMD [ "/url-resolver" ]