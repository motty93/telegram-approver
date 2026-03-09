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
```

## アーキテクチャ

単一ファイル（`main.go`）構成。主要な処理フロー：

1. `sendMessage()` — Telegram Bot APIでチャットにメッセージ送信
2. `getUpdates()` — ロングポーリングで更新を取得（2秒間隔）
3. `main()` — 送信したメッセージへの返信を監視し、`OK`/`いいえ` で終了コードを返す（0=承認, 1=拒否）

## 環境変数

- `TELEGRAM_TOKEN` — Telegram Bot APIトークン（必須）
- `TELEGRAM_CHAT_ID` — 送信先チャットID（必須）
