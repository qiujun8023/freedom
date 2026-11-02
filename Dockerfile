FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

ARG TARGETOS TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/freedom .

FROM scratch

COPY --from=build /out/freedom /freedom

ENV APP_CONFIG_DIR=/data

EXPOSE 8000

ENTRYPOINT ["/freedom"]
