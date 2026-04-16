---
name: monitor-wechat-push
title: Monitor WeChat Push
version: 2.0.0
description: 将监控脚本产出的消息写入 pending 文件，再通过 scheduler / agent 链路投递到企业微信会话（当前常用目标：wecom:GeYuHeng）。适用于股票、火车票、演唱会等监控任务的自动推送。
---

# Monitor WeChat Push

## 适用场景

当用户提到以下需求时，使用本 skill：

- 监控任务自动推送到微信
- 监控结果发到企业微信
- 股票监控 / 火车票监控 / 票务监控 自动通知
- 将脚本输出接入当前已跑通的微信通知链路
- 用 pending_notification.txt 方式做消息投递

> 注意：本 skill 这里的“微信推送”指当前工作区内已经验证可用的企业微信消息链路，而不是桌面微信客户端 UI 自动化发送。

---

## 当前推荐链路

当前监控任务的推荐推送方式不是操作桌面微信窗口，而是：

```text
监控脚本生成消息
  -> 写入 pending_notification*.txt
  -> scheduler / agent 定时检查
  -> 读取消息正文
  -> 投递到企业微信会话（如 wecom:GeYuHeng）
```

这是当前工作区内已经长期使用、相对稳定、可复用的方案。

---

## 标准模式

### 模式 A：监控脚本写 pending 文件

监控脚本不要直接耦合桌面自动化发送，而是优先把待发送内容写入文件，例如：

- `pending_notification.txt`
- `pending_notification_00340.txt`
- `pending_notification_combined.txt`

示例：

```python
from pathlib import Path

pending_file = Path('/root/.picoclaw/workspace/your-project/pending_notification.txt')
pending_file.write_text(content, encoding='utf-8')
```

如果需要带项目隔离，建议每个项目目录单独放自己的 pending 文件。

---

### 模式 B：发送前做去重/整理（推荐）

如果监控频率较高，建议增加状态文件，避免重复推送相同内容。

常见文件：

- `pending_notification.txt`：待发送正文
- `pending_notification_state.json`：去重状态
- `pending_notification_outbox.txt`：整理后的待投递正文（可选）

示例逻辑：

1. 检查 pending 文件是否存在
2. 读取正文
3. 若为空则删除
4. 与上次发送内容比较
5. 若重复则跳过并清理
6. 若不同则写入 outbox 或直接进入投递步骤

---

### 模式 C：由 scheduler / agent 投递到企业微信

推荐由 scheduler / agent 周期性执行发送动作，而不是让监控脚本自己直接调用不稳定的本地发送接口。

目标会话当前常用为：

- `channel = wecom`
- `chat_id = GeYuHeng`

也可以按项目需要改成其他已验证可用的会话。

---

## 推荐职责拆分

### 1. crawler / monitor
职责：采集、解析、生成消息正文

只负责：
- 抓取数据
- 判断是否有变化
- 组织中文推送正文
- 写入 pending 文件

不要在这个阶段做：
- 桌面微信窗口操作
- 鼠标键盘模拟
- 图像识别点击发送

---

### 2. sender / dispatcher
职责：读取 pending 文件并发出消息

只负责：
- 读取 pending 文件
- 去重
- 投递到企业微信会话
- 成功后清理 pending 文件

---

### 3. scheduler
职责：定时触发 sender / dispatcher

典型频率：
- 每 60 秒检查一次是否有待发送消息
- 每 1800 秒执行一次监控任务

---

## 推荐消息目标

当前工作区里，监控任务的已验证可用目标优先为：

```text
wecom:GeYuHeng
```

如果用户明确要求，也可以改为其他已验证通道；但在没有额外说明时，优先沿用当前稳定链路。

---

## 常见实现模板

### 模板 1：监控脚本写 pending 文件

```python
from pathlib import Path

BASE_DIR = Path('/root/.picoclaw/workspace/your-monitor')
PENDING = BASE_DIR / 'pending_notification.txt'


def send_wechat_message(content: str):
    PENDING.write_text(content, encoding='utf-8')
    return True
```

---

### 模板 2：去重整理脚本

```python
import json
from pathlib import Path

BASE_DIR = Path('/root/.picoclaw/workspace/your-monitor')
PENDING = BASE_DIR / 'pending_notification.txt'
STATE = BASE_DIR / 'pending_notification_state.json'
OUTBOX = BASE_DIR / 'pending_notification_outbox.txt'


def load_state():
    if not STATE.exists():
        return {}
    return json.loads(STATE.read_text(encoding='utf-8'))


def save_state(state):
    STATE.write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding='utf-8')


if PENDING.exists():
    content = PENDING.read_text(encoding='utf-8').strip()
    if content:
        state = load_state()
        if content != state.get('last_sent_content'):
            OUTBOX.write_text(content, encoding='utf-8')
            state['last_sent_content'] = content
            save_state(state)
    PENDING.unlink(missing_ok=True)
```

---

### 模板 3：由 agent/message 工具投递

在 Agent 环境中，应优先使用已经验证可用的消息投递链路，把正文发送到：

- channel: `wecom`
- chat_id: `GeYuHeng`

核心原则：
- 发送的是“监控正文”本身
- 不要只发送执行日志
- 不要把调度摘要误当成通知正文

---

## 与旧版桌面微信发送方案的区别

旧版桌面微信方案内容是：
- 聚焦微信窗口
- 截图识别
- 搜索联系人
- 粘贴消息
- 模拟快捷键发送

那属于：

```text
桌面微信 UI 自动化发送
```

而本 skill 现已统一改为：

```text
pending 文件 + scheduler / agent + 企业微信会话投递
```

这是两条完全不同的链路。

---

## 推荐命名约定

建议监控项目统一使用以下命名：

- `pending_notification.txt`
- `pending_notification_state.json`
- `pending_notification_outbox.txt`
- `run_and_prepare_xxx.py`
- `send_pending_xxx.py`
- `run_xxx_forever.sh`

这样更方便复用和排查。

---

## 使用原则

1. 优先复用现有稳定链路，不重新发明发送方式
2. 监控脚本负责产出消息，不直接绑死发送实现
3. 发送链路尽量做成可替换、可去重、可恢复
4. 目标渠道优先使用当前已验证可用的 `wecom:GeYuHeng`
5. 若用户明确要求“普通微信桌面发送”，那属于另一类 skill，不应与本 skill 混用

---

## 一句话总结

本 skill 的职责是：

> 把监控任务输出接入当前工作区已经跑通的“pending 文件 + scheduler/agent + 企业微信”推送链路，而不是操作桌面微信客户端发送消息。
