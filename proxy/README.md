# Mac (darwin arm64) 服务端

CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/proxy.server ./cmd/server

# Windows amd64 客户端

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/proxy.client.exe ./cmd/client

# http2 隧道

./proxy.server -transport http2 -listen 0.0.0.0:8443

# quic 隧道

./proxy.server -transport quic -listen 0.0.0.0:8443

# ssh2 隧道

./proxy.server -transport ssh2 -listen 0.0.0.0:2222

Windows client：

# ssh2

proxy.client.exe -transport ssh2 -server 192.168.1.100:2222 -proto socks5

# quic

proxy.client.exe -transport quic -server 192.168.1.100:8443 -proto socks5

proxy.client.exe -discover=true -transport quic -proto socks5

### 参数说明

-   `-transport`：隧道协议，支持 `http2` / `quic` / `ssh2`
-   `-proto`：本地代理协议，`socks5` / `connect`
-   `-server`：远端隧道地址，自动发现模式下留空
-   `-discover`：是否开启内网组播自动发现
-   `-discover-timeout`：发现超时时间（秒）
