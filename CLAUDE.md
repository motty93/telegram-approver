# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

Telegram Bot APIを使った承認フローツール。Telegramチャットにメッセージを送信し、ユーザーからの返信（`OK` で承認、`いいえ` で拒否）をポーリングで待機するCLIアプリケーション。

## 技術スタック

- Go 1.24.4
- 外部依存なし（標準ライブラリのみ）

## ビルド・実行

```bash
# ビルド
go build -o telegram-approver .

# 実行（環境変数が必須）
export TELEGRAM_TOKEN="your-bot-token"
export TELEGRAM_CHAT_ID="your-chat-id"
./telegram-approver                    # デフォルトメッセージで送信
./telegram-approver "カスタムメッセージ"  # カスタムメッセージで送信
./telegram-approver hook               # Claude Code PreToolUse hookモード
```

## アーキテクチャ

単一ファイル（`main.go`）構成。2つの動作モードがある：

### 直接モード（デフォルト）
1. `sendMessage()` — Telegram Bot APIでチャットにメッセージ送信
2. `getUpdates()` — ロングポーリングで更新を取得
3. `runApproval()` — 返信またはダイレクトメッセージを監視し、`OK`/`いいえ` で終了コードを返す（0=承認, 1=拒否）

### hookモード（`telegram-approver hook`）
Claude Code の PreToolUse hook として動作。stdinからJSON入力を読み取り、ツール種別に応じて承認判定：
- `Bash` — 危険コマンドパターン（`rm`, `sudo`, `deploy`, `terraform` 等）にマッチすればTelegram承認要求
- `Edit`/`Write` — memoryパス（`/.claude/projects/`）以外はTelegram承認要求
- その他 — 自動承認

### 承認判定
リプライ（`ReplyToMessage`）だけでなく、同一chat_idからのダイレクトメッセージもフォールバックとして受け付ける（スマートウォッチのクイックリプライ対応）。

## 環境変数

- `TELEGRAM_TOKEN` — Telegram Bot APIトークン（必須）
- `TELEGRAM_CHAT_ID` — 送信先チャットID（必須）
