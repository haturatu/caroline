# Caroline

Docker Engine で現在起動しているコンテナの stdout / stderr を、Cloud Logging に近い操作感で確認するための軽量な Web アプリです。Beszel のシンプルなサイドバーとダッシュボードの構成を参考にしています。

## 起動

Docker Compose を使う場合:

```sh
docker compose up -d --build
```

ブラウザで <http://localhost:8080> を開きます。Compose は `/var/run/docker.sock` を read-only mount でアプリへ渡し、起動中コンテナを毎回 Docker Engine API から取得します。Docker socket の所有グループはホストごとに異なるため、Compose コンテナは socket を読める root ユーザーで実行します。アプリケーション自身は Docker の GET API だけを呼び出します。

ローカルで Go から起動する場合:

```sh
npm ci
npm run build
go run .
```

`PORT` で待受ポート、`DOCKER_HOST` で Docker Engine の接続先を変更できます。通常は Unix socket の `/var/run/docker.sock` を使います。

フロントエンドは `src/app.ts` を TypeScript のみで実装しています。実行時に使うのはコンパイル済みの標準ブラウザ JavaScript だけで、React などの UI ランタイム依存はありません。

## Logs Explorer の構成

Google Cloud Logs Explorer の公開仕様と、ログイン済み Console を Chromium remote debugging 経由で観察した構成を参考にしています。Cloud Logging は Query pane、Fields pane、Timeline、Query results pane を同じ検索結果から更新し、`entries:list` のページングと `TailLogEntries` のストリーミングを分けています。

Caroline では Docker のログを Cloud Logging の `LogEntry` に近い形へ正規化し、次の構成で返します。

- `resource.type = "docker_container"` と container labels
- `logName`、`severity`、`timestamp`、`labels`、`textPayload` / `jsonPayload`
- Timeline の severity 別分布（Docker の tail 上での近似値）
- System Metadata / Frequent Fields の集計
- `AND` / `OR`、`=`, `!=`, `:`, `>=` などの軽量な Logging query 互換フィルター

## できること

- 起動中コンテナを自動検出し、コンテナごとにログを絞り込み
- stdout / stderr、時刻、コンテナ名、severity を正規化して一覧表示
- テキスト検索、Error / Warning / Info / Debug フィルター、15 分〜30 日の時間範囲
- 行をクリックして完全なメッセージ、コンテナ ID、イベント ID を表示
- 5 秒ごとの自動更新と Docker daemon 接続状態の表示
- Share Link、Query Syntax、Fields のフィールド値クリックによるクエリ追加

severity はログ本文の `ERROR`、`WARN`、`FATAL` などの文字列から UI 用に推定しています。ログ本文そのものはホストへ保存せず、画面を開いた時に Docker Engine から取得します。

## API

- `GET /api/health`
- `GET /api/status`
- `GET /api/explorer?duration=5m&limit=100&q=severity%20%3E%3D%20ERROR&severity=ERROR&stream=stderr&containers=id1&sort=desc`

Docker socket へのアクセス権がない環境では、アプリ自体は起動しますが、画面に接続エラーが表示されます。Compose では通常の Docker 環境で動作するよう socket をマウントしています。

認証機能はまだないため、8080 番ポートはローカルネットワーク内だけで公開してください。画面は外部フォントなどを使わず、単一バイナリと静的ファイルだけで動作します。
