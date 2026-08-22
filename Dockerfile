# syntax=docker/dockerfile:1
# DockerAdmin 全栈一体化镜像：前端镜像内构建 → Go embed → 精简运行时
# 跨平台：node/golang/alpine 官方镜像均支持 linux/arm64 + linux/amd64

# ---- Stage 1: 前端构建（镜像内 build；禁止 COPY 宿主机 dist —— devops 记忆规则） ----
FROM node:22-alpine AS fe-build
WORKDIR /fe
COPY frontend-admin/package.json frontend-admin/package-lock.json ./
RUN npm ci --no-audit --no-fund --registry=https://registry.npmmirror.com
COPY frontend-admin/ ./
RUN npm run build

# ---- Stage 2: Go 构建（embed 前端产物，单二进制交付） ----
FROM golang:1.25-alpine AS be-build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=fe-build /fe/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dockeradmin ./cmd/server

# ---- Stage 3: 运行时 ----
# 注：以 root 运行是有意为之 —— docker.sock 的 GID 随宿主发行版变化，非 root 无法可移植地访问
# （cAdvisor/Netdata 同策略）；已用 no-new-privileges + cap_drop ALL 收敛权限（见 compose）。
FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
 && apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=be-build /out/dockeradmin /app/dockeradmin
ENV TZ=Asia/Shanghai \
    PORT=8080 \
    DATA_DIR=/data \
    LOG_LEVEL=info \
    COLLECT_INTERVAL=2s \
    RETENTION_WINDOW=1h
EXPOSE 8080
CMD ["/app/dockeradmin"]
