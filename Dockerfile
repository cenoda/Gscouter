FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
# The browser build, which calculates results in visitors' browsers
# (internal/web/client.go), with the matching Go runtime glue.
RUN mkdir -p /out/static \
 && GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o /out/static/gscouter.wasm ./cmd/wasm \
 && gzip -9 -k /out/static/gscouter.wasm \
 && cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" /out/static/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
COPY --from=build /out/static /static
ENV GSCOUTER_STATIC=/static
EXPOSE 8080
ENTRYPOINT ["/server"]
