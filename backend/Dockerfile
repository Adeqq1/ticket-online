FROM oven/bun:1 AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lock* ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun run build

FROM golang:1.26 AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/ticket-api ./cmd/api && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/ticket-migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend-build /out/ticket-api /app/ticket-api
COPY --from=backend-build /out/ticket-migrate /app/ticket-migrate
COPY --from=frontend-build /src/frontend/dist /app/public
USER nonroot:nonroot
ENV HTTP_ADDR=:8080 STATIC_DIR=/app/public
EXPOSE 8080
ENTRYPOINT ["/app/ticket-api"]
