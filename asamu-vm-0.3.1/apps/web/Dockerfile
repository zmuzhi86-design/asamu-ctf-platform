# syntax=docker/dockerfile:1.7
ARG NODE_BUILD_IMAGE=node:22.18-alpine3.22
ARG NGINX_BASE_IMAGE=nginx:1.28-alpine

FROM ${NODE_BUILD_IMAGE} AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
ARG VITE_API_BASE=/api/v1
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_ENABLE_MOCK_API=false
RUN npm run build

FROM ${NGINX_BASE_IMAGE}
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
