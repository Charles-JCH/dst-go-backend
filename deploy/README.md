# 发布

平台服务器上的程序由 `deploy/update.sh` 更新，有两种触发方式：

- **自动**：推送到 `main` 后由 GitHub Actions 发布。后端仓库发布平台程序、Agent 和 `dst.sh`，前端仓库发布 `web/`。
- **手动**：在本机执行 `deploy/deploy.sh root@<平台服务器>`，前后端一起构建并发布。

`/opt/game-panel/config/config.yaml` 和 `storage/data/` 只在服务器上维护，发布不会改动。

## 配置自动发布（前后端两个仓库各配一次）

1. 在本机生成专用于发布的密钥，不设密码：

   ```bash
   ssh-keygen -t ed25519 -f ~/.ssh/game-panel-deploy -N "" -C github-actions-deploy
   ```

2. 把公钥加到平台服务器：

   ```bash
   ssh-copy-id -i ~/.ssh/game-panel-deploy.pub root@<平台服务器>
   ```

3. 记录平台服务器的主机公钥，发布时用它确认连到的是自己的服务器：

   ```bash
   ssh-keyscan -t ed25519 <平台服务器>
   ```

4. 在 GitHub 仓库 Settings → Secrets and variables → Actions 中添加：

   | 名称                 | 值                                       |
   | -------------------- | ---------------------------------------- |
   | `DEPLOY_HOST`        | 平台服务器 IP                            |
   | `DEPLOY_USER`        | SSH 用户名，非 root 需要免密 sudo        |
   | `DEPLOY_SSH_KEY`     | `~/.ssh/game-panel-deploy` 私钥的全部内容 |
   | `DEPLOY_KNOWN_HOSTS` | 第 3 步输出的那一行                      |

配置好后，也可以在仓库的 Actions 页面手动运行"发布后端"或"发布前端"。
