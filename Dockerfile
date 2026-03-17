# Stage 1: Build frontend
FROM node:20-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.23-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/server

# Stage 3: Final image with nginx + backend
FROM alpine:3.20

RUN apk add --no-cache nginx supervisor
RUN mkdir -p /run/nginx /var/log/supervisor

COPY --from=frontend-build /app/frontend/dist /usr/share/nginx/html
COPY --from=backend-build /app/server /usr/local/bin/server
COPY --from=backend-build /app/backend/migrations /app/migrations
COPY deploy/nginx.conf /etc/nginx/http.d/default.conf
COPY deploy/supervisord.conf /etc/supervisord.conf

EXPOSE 80

ENV PORT=8080

CMD ["supervisord", "-c", "/etc/supervisord.conf"]
