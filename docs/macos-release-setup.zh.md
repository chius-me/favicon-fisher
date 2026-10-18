# 可选：Apple 公证与自动更新 tap 的凭据配置

默认免费发布不需要本页中的任何凭据：使用 ad-hoc 签名、GitHub Release 和本地 `~/repo/github/homebrew-tap`，具体命令见 [免费发布流程](macos.md#public-release)。本页仅用于以后主动启用 Apple 公证或跨仓库自动 PR。Apple 公证和 tap 自动 PR 是两个独立选项。所有密码、证书私钥和 token 直接存入 GitHub Actions Secrets，不需要放进代码或聊天。

## 1. 确认 Apple Developer Program 会员

打开 [Apple Developer 账户](https://developer.apple.com/account)，确认 Membership details 中有有效的 Apple Developer Program 会员。免费账户和本机已有的 Apple Development 证书不能代替 Developer ID Application 发布证书。

若尚未加入，从 [Apple Developer Program](https://developer.apple.com/programs/) 申请；官方标价为每年 99 美元，实际当地金额以注册页面为准。个人会员本人就是 Account Holder。Apple 的 Developer ID 证书创建说明要求 Account Holder 权限。

## 2. 创建 Developer ID Application 证书

可以使用 Xcode 的账户设置创建本机证书；下面是 Apple 开发者网站的操作路径，适合确认每一步：

1. 打开 Mac 的“钥匙串访问”（Keychain Access）。在菜单中选择“钥匙串访问 → 证书助理 → 从证书颁发机构请求证书”。
2. 填写邮箱和通用名称，选择“存储到磁盘”，生成 `.certSigningRequest` 文件。这个过程会在本机生成私钥。
3. 打开 [Certificates](https://developer.apple.com/account/resources/certificates/list)，点击 `+`，选择 **Developer ID Application**，按页面要求上传刚生成的请求。我们的产物是 `.app` 和 `.dmg`，使用 Application 类型即可。
4. 下载 `.cer` 并在生成请求的这台 Mac 上双击安装。
5. 运行以下命令，确认出现完整身份名称：

```bash
security find-identity -v -p codesigning
```

需要的是形如 `Developer ID Application: YOUR NAME (TEAMID)` 的那一行。将引号内的完整名称作为 `MACOS_SIGNING_IDENTITY`，不要填前面的序号或十六进制指纹。

在钥匙串访问的“我的证书”中找到该证书，展开后应能看到私钥。选择该证书及对应私钥，使用“导出项目”，导出为 `.p12`，例如存为 `~/Downloads/DeveloperID.p12`，并设置一个导出密码。如果无法导出 `.p12`，先确认私钥就在当前钥匙串中；只下载 `.cer` 不包含私钥。

- 导出密码对应 `MACOS_CERTIFICATE_PASSWORD`。
- `.p12` 的 Base64 对应 `MACOS_CERTIFICATE_BASE64`。在终端运行下面的命令后，直接粘贴到 GitHub Secret 的值中：

```bash
base64 -i "$HOME/Downloads/DeveloperID.p12" | pbcopy
```

这个操作不把私钥内容打印到终端。证书包请保存在个人安全位置，不要放进 Git 仓库。

参考：[Apple 创建 Developer ID 证书](https://developer.apple.com/help/account/certificates/create-developer-id-certificates/)、[创建证书请求](https://developer.apple.com/help/account/certificates/create-a-certificate-signing-request)、[GitHub 在 macOS runner 中安装证书](https://docs.github.com/en/actions/how-tos/deploy/deploy-to-third-party-platforms/sign-xcode-applications)。

## 3. 准备公证凭据

公证由发布脚本自动提交，不需要手工上传应用。当前流程使用以下三个值：

| Secret | 填什么 |
| --- | --- |
| `APPLE_ID` | 上面开发者账户的登录邮箱 |
| `APPLE_TEAM_ID` | Developer 账户 → Membership details 中的 10 位 Team ID |
| `APPLE_APP_PASSWORD` | 为公证生成的 App-Specific Password |

生成专用密码：登录 [Apple Account](https://account.apple.com/) → “登录与安全” → “App 专用密码” → 生成，名称可用 `Favicon Fisher Notarization`。账户需要已启用双重认证。将生成的密码填入 `APPLE_APP_PASSWORD`；不要填写 Apple 账户的普通登录密码。

修改或重置 Apple 账户主密码会撤销已有的 App 专用密码；此后重新生成并更新 GitHub Secret 即可。

参考：[Apple App 专用密码](https://support.apple.com/en-us/102654)、[查找 Team ID](https://developer.apple.com/help/glossary/team-id/)。

## 4. 可选：为 CI 自动 PR 创建 Homebrew tap token

打开 [Fine-grained token 创建页面](https://github.com/settings/personal-access-tokens/new)，填写：

| 字段 | 设置 |
| --- | --- |
| Token name | `favicon-fisher-tap-release` |
| Resource owner | `chius-me` |
| Expiration | 例如 90 天；到期前替换 Secret |
| Repository access | **Only select repositories**，只选 `homebrew-tap` |
| Repository permissions → Contents | **Read and write** |
| Repository permissions → Pull requests | **Read and write** |
| Metadata | 保留自动附带的只读权限 |

生成后复制 token，保存为 `HOMEBREW_TAP_TOKEN`。该 token 的目标仓库是 `homebrew-tap`，但 Secret 放在运行发布工作流的 **favicon-fisher** 仓库。

参考：[GitHub 创建 fine-grained token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)、[权限说明](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens)。

## 5. 保存到源仓库，再启用

打开 [favicon-fisher 的 Actions Secrets](https://github.com/chius-me/favicon-fisher/settings/secrets/actions)，选择 **New repository secret**，逐项创建以下七个名称：

| 名称 | 内容 |
| --- | --- |
| `MACOS_CERTIFICATE_BASE64` | `.p12` 经 Base64 编码后的文本 |
| `MACOS_CERTIFICATE_PASSWORD` | 导出 `.p12` 时设置的密码 |
| `MACOS_SIGNING_IDENTITY` | 完整的 `Developer ID Application: … (TEAMID)` |
| `APPLE_ID` | Apple 账户登录邮箱 |
| `APPLE_APP_PASSWORD` | App 专用密码 |
| `APPLE_TEAM_ID` | 会员的 Team ID |
| `HOMEBREW_TAP_TOKEN` | 上一步的 fine-grained token |

启用 Apple 公证时保存前六项即可，本地更新 tap 不需要第七项。准备好 Apple 凭据后，打开同一设置中的 [Variables](https://github.com/chius-me/favicon-fisher/settings/variables/actions)，添加 Repository variable：

```text
MACOS_NOTARIZE = true
```

这一步只让未来发布使用 Apple 公证，不会立即发布。若还要自动创建 tap PR，另设 `HOMEBREW_TAP_AUTOMATION=true` 并配置第七项 token；默认本地更新无需该设置。还需要先把实现合入源仓库，再推送一个没有使用过的稳定版本 tag（`vMAJOR.MINOR.PATCH`）。发布流会完成构建、签名、公证、上传和 tap PR；不要重用旧 tag 或替换已经发布的 DMG。

可以用以下只显示名称/变量值的命令检查配置完整性：

```bash
gh secret list --repo chius-me/favicon-fisher
gh variable list --repo chius-me/favicon-fisher
```

完整流程和失败重试见 [macOS 发布文档](macos.md#public-release)。
