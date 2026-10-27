# Build stage
FROM node:24.22.1-alpine AS builder
RUN apk upgrade --no-cache
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG REACT_APP_API_BASE_URL
ENV REACT_APP_API_BASE_URL=$REACT_APP_API_BASE_URL
RUN npm run build

# Runtime stage
FROM nginx:stable-alpine
# Refresh installed runtime packages even when the upstream base digest lags.
# Alpine 3.24 fixes Brotli CVE-2026-111497 and curl HTTP/2 UAF CVE-2026-92392.
RUN apk upgrade --no-cache
COPY --from=builder /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]