# 构建阶段
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git

# 复制go.mod和go.sum文件，利用层缓存
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源码
COPY . .

# 构建
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o lobsterhub .

# 运行阶段
FROM alpine:3.19

# 创建非特权用户
RUN addgroup -S lobsterhub && adduser -S lobsterhub -G lobsterhub

WORKDIR /app

# 安装运行依赖
RUN apk add --no-cache ca-certificates

# 复制二进制
COPY --from=builder /app/lobsterhub .

# 创建数据目录并设置权限
RUN mkdir -p /app/data && chown -R lobsterhub:lobsterhub /app

# 环境变量
ENV PORT=8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/health || exit 1

EXPOSE 8080

# 使用非特权用户
USER lobsterhub

ENTRYPOINT ["./lobsterhub"]
CMD ["-addr", ":8080"]
