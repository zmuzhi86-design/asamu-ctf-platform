# asamu 第一次发布到 GitHub：完整操作指南

## 一、发布前先做四件事

### 1. 不要把压缩包当作仓库源码

应先解压，再把解压后的目录提交到 GitHub。仓库首页才能直接显示源码、README 和目录结构。

### 2. 使用新的根目录 `.gitignore`

必须忽略：

```text
.env.docker
deployment-credentials.txt
真实证书和私钥
数据库备份
日志
node_modules
构建产物
```

### 3. 检查真实秘密

在项目目录执行：

```bash
grep -RInE \
  'POSTGRES_PASSWORD=|JWT_ACCESS_SECRET=|FLAG_HMAC_SECRET=|DEV_ADMIN_PASSWORD=' \
  . \
  --exclude='.env.docker.example' \
  --exclude-dir='.git'
```

如果输出包含真实密码、Token 或密钥，先删除或替换，再提交。

### 4. 决定许可证

当前项目没有 LICENSE。

- 所有代码和素材都是你自己的，并希望别人自由使用：可考虑 MIT。
- 希望衍生项目继续开源：可考虑 GPL-3.0。
- 希望提供更明确的专利授权：可考虑 Apache-2.0。
- 暂时不确定：先不添加许可证，不要随便选择。

## 二、编辑 README

将发布包里的：

```text
README.md
```

覆盖项目原来的根目录 `README.md`。

然后搜索并替换：

```text
YOUR_GITHUB_USERNAME
```

替换为你的 GitHub 用户名。

### VS Code 编辑方法

1. 解压项目。
2. 用 VS Code 打开项目根目录。
3. 点击根目录的 `README.md`。
4. 修改 Markdown 文本。
5. 按 `Ctrl + Shift + V` 预览效果。
6. 按 `Ctrl + S` 保存。

常用 Markdown：

```markdown
# 一级标题
## 二级标题

**粗体**

- 列表项目

[显示文字](链接)

![图片说明](docs/images/demo.png)

`行内代码`

```bash
命令
```
```

## 三、创建 GitHub 空仓库

推荐设置：

```text
Repository name: asamu-ctf-platform
Description: 使用 PROJECT_COPY.md 中的 Description
Visibility: Public
```

因为本地已经有 README 和 `.gitignore`，创建仓库时不要勾选自动生成 README、`.gitignore` 或许可证。

## 四、Windows 上上传源码

### 方案 A：Git Bash / PowerShell

先进入解压后的项目目录：

```bash
cd 你的路径/asamu-vm-0.3.1
```

把发布包里的 README 和 `.gitignore` 放到项目根目录后执行：

```bash
git init
git branch -M main
git add .
git status
git commit -m "feat: publish asamu v0.3.1"
git remote add origin https://github.com/YOUR_GITHUB_USERNAME/asamu-ctf-platform.git
git push -u origin main
```

把 `YOUR_GITHUB_USERNAME` 替换成你的用户名。

### 方案 B：GitHub Desktop

1. 打开 GitHub Desktop。
2. 选择 **Add an Existing Repository from your Hard Drive**。
3. 选择解压后的项目目录。
4. 如果提示不是 Git 仓库，选择创建仓库。
5. Summary 填：`feat: publish asamu v0.3.1`。
6. 点击 Commit to main。
7. 点击 Publish repository。
8. 取消 Private，发布为 Public。

## 五、以后怎么修改

### 本地修改后上传

```bash
git status
git add .
git commit -m "docs: update deployment guide"
git push
```

常用提交说明：

```text
feat: add a new feature
fix: fix a bug
docs: update documentation
style: adjust UI styles
refactor: refactor code
test: add or update tests
chore: update scripts or dependencies
```

### 在 GitHub 网页直接修改 README

1. 进入仓库。
2. 打开 `README.md`。
3. 点击右上角铅笔图标。
4. 修改内容并切换 Preview 检查。
5. 点击 Commit changes。
6. 填写说明后提交。

小改动可以网页编辑；代码和多文件修改建议使用 VS Code。

## 六、发布 v0.3.1 Release

1. 进入仓库首页。
2. 点击右侧 Releases。
3. 点击 Draft a new release。
4. 创建标签：`v0.3.1`。
5. Release title 填：`asamu v0.3.1 — 首个公开版本`。
6. 将 `GITHUB_RELEASE_v0.3.1.md` 内容复制到说明框。
7. 上传原始发布包：`asamu-vm-0.3.1.tar.gz`。
8. 点击 Publish release。

## 七、仓库首页继续完善

进入仓库右侧 About 区域，添加：

- Description
- 项目网站地址（将来有演示站再填）
- Topics

建议 Topics：

```text
ctf cybersecurity security-training docker docker-compose golang react typescript postgresql redis pwn web-security ctf-platform
```

## 八、推荐的第一批提交

```text
feat: publish asamu v0.3.1
docs: improve GitHub README
docs: add deployment and dynamic challenge guides
chore: add root gitignore
```

## 九、出现常见错误时

### `remote origin already exists`

```bash
git remote -v
git remote set-url origin https://github.com/YOUR_GITHUB_USERNAME/asamu-ctf-platform.git
```

### `src refspec main does not match any`

通常是还没有提交：

```bash
git add .
git commit -m "feat: publish asamu v0.3.1"
git branch -M main
git push -u origin main
```

### 登录时密码失败

GitHub 命令行 HTTPS 推送不能使用账号登录密码，应使用 Personal Access Token、Git Credential Manager、GitHub Desktop 或 SSH。
