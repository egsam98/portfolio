FROM golang:1.18-alpine as BUILDER

WORKDIR /portfolio

RUN apk add git
ARG GITLAB_USER
ARG GITLAB_TOKEN
RUN git config --global url."https://${GITLAB_USER}:${GITLAB_TOKEN}@gitlab.com".insteadOf "https://gitlab.com"

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

ARG VERSION
RUN CGO_ENABLED=0 go build -o portfolio -ldflags "-X main.version=${VERSION}" *.go

FROM alpine
COPY --from=BUILDER /portfolio /
CMD ["/portfolio"]