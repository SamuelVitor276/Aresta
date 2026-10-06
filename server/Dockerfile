# ---- build: compila um binário estático ----
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aresta-server .

# ---- runtime: imagem mínima (sem shell), rodando como usuário sem privilégios ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/aresta-server /aresta-server
EXPOSE 8080
ENTRYPOINT ["/aresta-server"]
