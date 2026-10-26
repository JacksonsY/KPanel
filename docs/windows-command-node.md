# Windows 命令行轻量节点

本候选让 Windows 作为 `light_node` 接入 Linux KPanel 中心，提供实时与历史监控、文件管理、
PowerShell 命令行终端。Panel/Agent 仍只在 Linux 运行。没有 RDP、远程桌面、账户创建、
远程端口转发或 Windows 系统管理适配器。

## 接入与运行

在集群添加主机或批量添加中选择 Windows，生成并复制同一授权对应的 PowerShell 命令，
在目标节点的管理员 PowerShell 中执行。单台命令沿用五分钟一次性授权；批量沿用现有
次数、期限、撤销和重试身份。命令先核对固定版本官方 HTTPS bootstrap 的内置 SHA-256，
再由 bootstrap 校验同一 Release 的安装器和二进制。凭据通过进程内调用和有界 stdin 传递，
不写入子进程参数、环境或服务注册参数。粘贴的命令仍可能进入管理员自己的 PowerShell 历史。

发布前需要同一候选的中心与 Windows 附件；当前公共版本没有这些新附件时，生成命令不能用来
证明安装成功。构建入口为 `scripts/windows-node-release.ps1`，原生验证入口为
`scripts/verify-windows-node.ps1`；两者只生成仓外产物，不发布。公共附件未使用代码签名证书，
官方 HTTPS 和 SHA-256 校验不能被描述为证书发布者认证。

代码目标为 Windows 10 1809 / Server 2019 及以后，`amd64` / `arm64`；实际支持仍以各环境
验收为准。ConPTY 不可用、权限或可信系统 PowerShell 检查失败时明确拒绝安装终端能力。
不依赖桌面图形会话，默认启动系统自带 PowerShell，使用 UTF-8、无用户 profile 和有界 ConPTY；
Job Object 保证关闭时收回整棵进程树。批量执行复用现有 PowerShell 包装与退出码契约。

## 权限、资源和恢复

- 三个 SCM 服务：`KejilionNode` 使用独立低权限虚拟服务账户读取遥测；
  `KejilionNodeFile`、`KejilionNodeTerminal` 使用 LocalSystem。历史采样在文件服务中独立于中心重连运行。
  不使用 NSSM，不新增监听端口；只使用既有 HTTPS、HMAC、Noise 中继和中心远程访问策略。
- 节点配置、终端私钥与安装暂存使用明确 DACL；秘密从创建时起封闭，拒绝不可信父目录与 reparse point。
  旧预览 Windows 身份继续按原撤销策略清理；仅新接入记录的 `windowsCommand` 标记允许管理，
  上报不能将已接入 Windows 身份改为 Linux，也不能使旧身份恢复授权。
- 遥测使用原生 CPU、内存、固定磁盘、默认路由网卡和运行时间 API。Windows 不提供的
  `load`、`swap`、`diskIO`、`networkConnections` 标记为不可用，历史界面不伪造实测零值。
  未实现 Windows 自定义网卡选择，不承诺公网地址查询或所有虚拟适配器的特殊拓扑。
- 历史复用现有 JSONL、小时聚合、查询并发、保留期和磁盘配额，不复制到中心形成第二套历史。
  文件与终端沿用现有传输、会话、超时、背压与输出限额。文件虚拟根 `/` 列出固定盘符，
  `/C/...` 映射到 `C:\...`；拒绝 ADS、设备名、路径穿越、junction、符号链接和硬链接绕过。
- 文本保存、覆盖上传和跨卷移动保持源 DACL；普通复制沿用目标继承。POSIX chmod 不适用，
  界面不提供八进制权限修改。私有归档暂存必须从创建时设置 DACL，并核对所持目录身份。
  原生测试覆盖常规读写、覆盖、大小写重命名、跨卷移动、占用失败和中文内容；
  断电后的目录传输/归档恢复与所有 Windows 文件系统仍需专项验收。
- 服务配置恢复重启；计划任务检查稳定版更新。更新先验证固定版本附件，再用有界事务、摘要、
  健康检查和回滚恢复服务。卸载只遍历两个固定 KnownFolder 下的节点目录，拒绝 reparse point。
  本机开发验证不安装服务，不执行真实更新或卸载；这些生命周期必须在隔离 Windows 实例验收。

## 兼容与回滚

新增字段是 light-v1 可选元数据，不变更 Linux 安装命令、脚本协议或内置脚本。
`scriptLinkageState=not-required`（无需发布脚本（不适用））；脚本仍固定
`614d58cee228007012a2ec602cbdc8dbaeb7950d`，SHA-256
`ec6d52da028ad6c6145fbb9c6796cd1548104b288f9234551c3b3d800b43eec9`。

中心与 Windows Node 成对升级。旧中心不认识新能力时节点拒绝管理，不猜测支持；回退前保存
升级前中心状态和节点配置，旧中心可能拒绝新字段或清理 Windows 身份。回滚代码不会撤销已经
执行的命令或文件写入；在途任务应先结束或核对。旧 Windows 节点需重新生成接入授权。
新发布冻结 19 项附件；旧控制器冻结的 15 项清单保持原字节、原名称和原摘要恢复验证。

## 参考与验收边界

2026-10-26 核对 [Komari Agent](https://github.com/komari-monitor/komari-agent)：其 Windows Agent、
PowerShell 安装和 ConPTY 命令行路径用于复核核心旅程。实现使用 KPanel 自有协议和原生 SCM、
现有历史/文件/终端组件；不引入 Komari Token、NSSM、端口或依赖，不恢复历史 RDP 实现。
源码与本机原生自动测试、Linux 回归、UI Mock、隔离安装/重启、公开附件、生产部署是不同证据层。
候选验证回执记录实际通过情况；本文的实现说明不构成 Server/ARM64 实机、长期性能或发布承诺。
