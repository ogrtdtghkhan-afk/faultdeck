# 使用 Docker 部署 FaultDeck

[English deployment guide](deployment.md)

这里部署的是可转发真实请求的代理与控制台，包含认证和工作区持久化。静态网站上的交互演示与此服务分开。请使用当前源码仓库，旧版 v0.1.0 压缩包尚未包含部署文件。

## 启动服务

先安装 Docker 和 Compose v2 插件；Docker Desktop 请选择 Linux 容器模式。宿主机无需安装 Go 或 Node.js。

```sh
git clone https://github.com/ogrtdtghkhan-afk/faultdeck.git
cd faultdeck
cp .env.example .env
```

Windows PowerShell 将最后一行替换为 `Copy-Item .env.example .env`。

编辑 `.env`，把 **`FAULTDECK_ADMIN_PASSWORD`** 填成自己生成的随机密码，至少 12 个字符。没有可直接使用的默认密码。用户名默认是 `admin`。可以另设同样至少 12 个字符的 `FAULTDECK_PROXY_TOKEN`；留空时，代理令牌使用管理员密码。`.env` 已排除在 Git 和 Docker 构建上下文之外，请妥善保存。

```sh
docker compose up --build -d
docker compose ps
```

等 `faultdeck` 显示 healthy 后，打开 **http://127.0.0.1:7331**，在浏览器的 HTTP Basic Auth 登录框输入 `admin` 和自设密码。业务代理地址是 **http://127.0.0.1:7332**。

镜像使用 [Docker 官方 Go 构建镜像](https://hub.docker.com/_/golang)和带 CA 证书的 [Distroless nonroot 运行镜像](https://github.com/GoogleContainerTools/distroless)。进程使用 UID/GID 65532，根文件系统只读，`/data` 通过命名卷提供可写存储。默认只发布宿主机回环地址的两个端口，内置演示服务不对外发布。

## 接入自己的真实 API

在控制台修改 **Upstream**，填写容器能够访问的后端地址并保存。首次安装可先试内置演示，确认服务正常后再切换到自己的开发或测试接口。

| 后端位置 | 上游地址示例 |
| --- | --- |
| 同一 Docker 网络的其他容器 | `http://your-api-service:8000` |
| Docker 宿主机上的服务 | `http://host.docker.internal:8000` |
| 可达的开发服务器 | `https://dev-api.example.com` |

容器内的 `127.0.0.1` 指向容器自己，不是宿主机或其他容器。Compose 已配置 `host.docker.internal` 的 host-gateway 映射；Linux 宿主机上的后端仍须监听 Docker 网桥可访问的地址，只绑定宿主机 `127.0.0.1` 的服务可能无法连接。使用服务名访问其他容器前，需要让它们加入同一 Docker 网络。

应用发往 7332 端口的每个请求需要携带 **`X-FaultDeck-Token`**。该令牌在转发前会移除，业务自身的 **`Authorization`** 请求头会保留给真实后端。

代理令牌只能发送给 FaultDeck。请关闭会保留该自定义请求头的客户端自动重定向，避免后端的跳转响应把令牌带到其他服务器。Node.js `fetch` 使用 `redirect: "manual"`；附带的重试客户端已经拒绝跟随重定向。建议设置独立代理令牌，将客户端凭据与管理员密码分开。

Bash 示例：在提示处输入代理令牌；如果没有设置独立令牌，则输入管理员密码。

```sh
read -r -s -p "Proxy token: " proxy_token
printf '\n'
curl -i -H "X-FaultDeck-Token: $proxy_token" http://127.0.0.1:7332/api/orders
unset proxy_token
```

PowerShell 示例：

```powershell
$credential = [System.Net.NetworkCredential]::new('', (Read-Host 'Proxy token' -AsSecureString))
Invoke-WebRequest http://127.0.0.1:7332/api/orders -MaximumRedirection 0 -Headers @{'X-FaultDeck-Token' = $credential.Password}
Remove-Variable credential
```

将请求路径换成真实后端提供的接口。为该路径创建 **Fail twice** 规则，连续请求三次，应该看到两次注入的 503，然后是后端的真实响应。控制台内置试验区会在服务端自动附加代理令牌。

浏览器应用应通过开发服务器或 BFF 的同源路由接入，并在该服务端添加代理令牌。不要把管理员密码或代理令牌写入公开的前端代码。FaultDeck 不提供通用的浏览器跨域 API，也不会自动给注入的错误响应添加 CORS 头。

## 重启后保留什么

`faultdeck-data` 命名卷保存上游地址、故障总开关和规则。通过 UI/API 修改配置后会保存到卷中。请求日志、统计和规则执行计数只在内存中；服务重启后，“失败两次”的计数会重新开始。

```sh
docker compose restart faultdeck
```

`docker compose down` 会保留命名卷。再次启动时沿用原 Compose 项目名和目录，才能使用同一份工作区。删除卷会清空工作区，因此普通停止不要加 `--volumes`。优先使用随附的命名卷；如果改为宿主机目录绑定挂载，该目录必须允许 UID/GID 65532 写入。

认证信息来自 `.env`，不存入工作区。修改密码、令牌或公开访问地址后，用 `docker compose up -d --force-recreate` 应用新环境；仅 restart 不会替换已有容器的环境变量。

## 从另一台电脑访问

默认宿主机端口仍仅本机可访问。个人使用远程 Docker 主机时，可以通过 SSH 转发两个端口：

```sh
ssh -L 7331:127.0.0.1:7331 -L 7332:127.0.0.1:7332 user@your-server
```

然后使用相同的本地网址和认证信息；使用期间保持隧道连接。

需要多人通过 HTTPS 访问时，在 Docker 宿主机部署 TLS 反向代理，将两个不同域名分别转发到 `127.0.0.1:7331` 和 `127.0.0.1:7332`。在 `.env` 中填写实际的公开地址并重建容器：

```dotenv
FAULTDECK_UI_ORIGIN=https://faultdeck.example.com
FAULTDECK_PROXY_URL=https://faults.example.com
```

这些配置用于声明访问地址，不会自动创建 DNS 或 TLS 证书。反向代理应保留请求 Host 和认证头；每个域名对应正确端口，不要增加路径前缀。`--ui-origin` 必须与浏览器实际使用的协议、域名和端口一致。保留 Basic Auth 和代理令牌，在本机或 SSH 隧道之外使用 HTTPS。

## 运维与排查

```sh
docker compose logs --tail=100 faultdeck
docker compose exec -T faultdeck /faultdeck --healthcheck
```

控制端口的 `/healthz` 是不需要认证、只返回最少信息的健康检查入口。可执行文件的 healthcheck 访问内部监听地址，成功退出 0，失败退出 1，不依赖浏览器认证或公开域名。自行改变容器内部 UI 端口时，也应给 healthcheck 传入匹配的 `--ui-port`。

| 现象 | 检查内容 |
| --- | --- |
| Compose 提示缺少密码 | 填写 `.env` 中的密码，至少 12 个字符 |
| 浏览器登录失败 | 检查用户名、密码，修改后重建容器 |
| 代理返回 401 | 添加 `X-FaultDeck-Token`；它与业务 `Authorization` 分开 |
| 代理返回 502 | 检查容器到上游的网络，以及上游 TLS 证书 |
| UI 提示来源或 Host 错误 | `FAULTDECK_UI_ORIGIN` 须匹配浏览器地址的协议、域名和端口 |
| 无法保存工作区 | 检查卷权限和磁盘空间；运行 UID 为 65532 |
| 宿主机端口被占用 | 同时修改 `.env` 的端口和对应访问 URL |

更新时审阅新源码后运行 `docker compose up --build -d`，保留已有卷。FaultDeck 面向开发、测试环境，会有意让经过代理的流量发生故障。

## 真实容器验收

安装 Docker 和 Python 3 后，在仓库根目录执行：

```sh
python3 scripts/deployment-smoke.py
```

Windows 可把 `python3` 换成 `python`。脚本构建真实镜像，创建隔离的 Compose 项目和独立上游容器，检查认证、真实 POST 正文与查询参数、业务认证保留、令牌剥离、503/503/200、延迟、超时、TCP 断连、非 root 健康检查，以及重建容器后的持久化。它使用随机凭据和端口，结束后只清理自己的测试容器和卷。

[Deployment 工作流](https://github.com/ogrtdtghkhan-afk/faultdeck/actions/workflows/deployment.yml)执行同一套测试，并保留 JSON 验收记录。应以对应提交的实际运行结果判断部署是否通过；本机 Go 测试或静态网站预览不能代替容器验收。
