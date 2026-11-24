# Stage 1: Build frontend
FROM node:24-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# Stage 3: Runtime — binary and frontend/dist as siblings under WORKDIR
FROM alpine:3
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-build /server ./server
COPY --from=frontend-build /app/frontend/dist ./frontend/dist
EXPOSE 80
ENV PORT=80
# Inside a container the server must answer outside its own loopback (ADR 048
# made 127.0.0.1 the default for the VM, where Caddy is in front).
ENV LISTEN_HOST=0.0.0.0
CMD ["./server"]
