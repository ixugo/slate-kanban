# Slate 看板的镜像：三阶段构建，最终镜像里只有一个带前端的单文件程序。

# 阶段一：构建前端。vite 把产物写到 ../internal/web/spa/dist，阶段二再把它嵌进程序。
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 阶段二：编译后端。依赖都是纯 Go（SQLite 用 glebarez/sqlite），关闭 cgo 得到静态程序。
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/web/spa/dist ./internal/web/spa/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -tags embed_web,nomsgpack -trimpath \
    -ldflags="-s -w -X main.buildVersion=${VERSION} -X main.release=true" \
    -o /out/slate .

# 阶段三：运行镜像。用普通用户运行；数据库和日志放在 /data，建议挂卷保存。
FROM alpine:3.22
ENV TZ=Asia/Shanghai
RUN apk --no-cache add ca-certificates tzdata \
    && adduser -D -u 10001 slate \
    && mkdir -p /app/configs /data \
    && chown -R slate:slate /app /data
WORKDIR /app
COPY --from=build /out/slate /app/slate
# 容器内要监听 0.0.0.0，数据放 /data；对外是否开放由 docker 的端口映射决定
COPY --chown=slate:slate configs/config.docker.toml /app/configs/config.toml
USER slate
VOLUME /data
EXPOSE 7070
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -q -O /dev/null http://127.0.0.1:7070/health || exit 1
CMD ["./slate", "-conf", "/app/configs"]
