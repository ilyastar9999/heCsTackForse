FROM golang:1.26-alpine AS build
WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/smoke-flag main.go

FROM scratch
COPY --from=build /out/smoke-flag /smoke-flag
EXPOSE 8080
ENTRYPOINT ["/smoke-flag"]
