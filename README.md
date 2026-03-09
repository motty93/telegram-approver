# telegram-approver

Telegram Bot APIを使った承認フローCLIツール。Telegramチャットにメッセージを送信し、ユーザーからの返信を待って承認/拒否を判定します。

## セットアップ

### 前提条件

- Go 1.24.4以上
- Telegram Botトークン（[BotFather](https://t.me/botfather)で作成）
- 送信先のチャットID

### インストール

```bash
go install github.com/motty93/telegram-approver@latest
```

またはソースからビルド:

```bash
git clone https://github.com/motty93/telegram-approver.git
cd telegram-approver
go build -o telegram-approver .
```

## 使い方

### 環境変数

| 変数名 | 説明 | 必須 |
|--------|------|------|
| `TELEGRAM_TOKEN` | Telegram Bot APIトークン | Yes |
| `TELEGRAM_CHAT_ID` | 送信先チャットID | Yes |

### 実行

```bash
export TELEGRAM_TOKEN="your-bot-token"
export TELEGRAM_CHAT_ID="your-chat-id"

# デフォルトメッセージで送信
./telegram-approver

# カスタムメッセージで送信
./telegram-approver "デプロイを承認してください"
```

### 承認フロー

1. 指定したチャットにメッセージが送信される
2. ユーザーがそのメッセージに**返信**する
3. 返信内容に応じて終了コードが返る

| 返信 | 結果 | 終了コード |
|------|------|-----------|
| `OK` | 承認 | 0 |
| `いいえ` | 拒否 | 1 |

## ライセンス

MIT
