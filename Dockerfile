# 构建阶段
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git

# 复制源码
COPY . .

# 构建
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o lobsterhub .

# 运行阶段
FROM alpine:3.19

WORKDIR /app

# 安装运行依赖
RUN apk add --no-cache ca-certificates

# 复制二进制
COPY --from=builder /app/lobsterhub .

# 创建数据目录
RUN mkdir -p /app/data

# 环境变量
ENV PORT=8080

EXPOSE 8080

ENTRYPOINT ["./lobsterhub"]
CMD ["-addr", ":8080"]
