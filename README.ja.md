# Caroline
<img width="1341" height="629" alt="top" src="https://github.com/user-attachments/assets/d89ad9db-64fa-45c4-93af-3e7329bdca6e" />

Caroline は、複数の Docker ホストに配置した Agent からログを収集し、Hub で横断検索する軽量なログ管理システムです。Google Cloud Logs Explorer の情報設計に着想を得ていますが、クエリ構文やデータモデルは Docker ログ向けの小さな subset です。

<img width="647" height="621" alt="image" src="https://github.com/user-attachments/assets/22a98400-9a31-4c45-a70f-fa00ac073804" />  

メモリ追加購入しなくて済むね！やったねたえちゃん！

## 作った理由
自分のサーバでは主に全てコンテナ管理しているが統合したログ確認などが出来ず、外出先から確認するときは `Beszel` での単体のコンテナログを確認していました。ただしなかなかログ管理という観点だとちょっと厳しかったので個人的に使いやすい GCP の Cloud Logging の機能である `Logs Explorer` のような UI/UX で操作できればいいなと思い作成に至りました。  
アプリ名 / プロジェクト名 はたまたま The Velvet Underground の Caroline を聞いていただけでつけたもの。  

## 主な特徴

- Hub が正規化したログを SQLite に保存
- 各ホストの read-only Docker Engine を監視する `caroline-agent`
- batch、再送、disk spool、重複排除による at-least-once 配送
- Node 単位の検索、コンテナ grouping、Agent の online / offline 表示
- コンテナ、stdout / stderr、severity、時間範囲での絞り込み
- 全フィールド検索と Caroline Query Syntax による検索
- Timeline、Fields 集計、ログ詳細 drawer
- SSE による新着ログの Streaming 表示
- しきい値・時間枠・クールダウンに対応したログアラートと、Discord / Slack / ntfy / Microsoft Teams / 汎用 Webhook 通知
- URL に検索条件を保存する Share Link
- ダーク / ライトテーマ、モバイル用ナビゲーション
- Agent から Docker Engine へ読み取り専用アクセス（Hub に Docker socket は不要）

## 起動

### Docker Compose

```sh
docker compose up -d --build
```

ブラウザで <http://localhost:8080> を開きます。

通常の Compose 起動では Hub のみを起動し、`/var/run/docker.sock` は Hub にマウントしません。Hub はログとidentityを `caroline-data` volume に保存します。リバースプロキシ配下で運用する場合は、Nodes画面が正しいEnrollment URLを生成できるよう`CAROLINE_PUBLIC_URL`を設定します。

```sh
CAROLINE_PUBLIC_URL=https://caroline.example.com docker compose up -d --build
```

各Dockerホストではリポジトリをcloneし、分離されたAgent用Composeを起動します。HubのNode画面からEnrollment URLをコピーし、初回起動時だけ指定します。

```sh
git clone https://github.com/haturatu/caroline
cd caroline
CAROLINE_ENROLL_URL='https://caroline.example.com/api/v1/agent/enroll/...' \
  docker compose -f compose.agent.yml up -d --build

# 2回目以降は永続volumeを使うためURLなしで再起動できます
docker compose -f compose.agent.yml up -d
```

Agent用ComposeはAgent imageをGHCRから取得せず、対象ホスト上でローカルbuildします。Agentは`identity.json`、Hub pin、未送信spoolを`caroline-agent-data` volumeに保存します。Enrollment URLは単回利用で、初回登録後は保存済みidentityと`hub.json`を使ってHubへ再接続します。Compose serviceには`caroline.collect=false` labelを設定し、Agent自身の診断ログを収集対象から除外しています。同じlabelを付けたコンテナもログ収集対象外になります。

停止する場合:

```sh
docker compose down
```

### ローカルで起動

必要なもの:

- Go 1.26 以上
- Node.js 22 以上と npm
- 接続可能な Docker Engine（`caroline-agent` 側で必要）

```sh
npm ci
npm run build
go run ./cmd/caroline

# 各 Docker ホストで、CAROLINE_HUB_URL 等を設定した後に起動
go run ./cmd/caroline-agent
```

デフォルトでは <http://localhost:8080> で待ち受けます。

## 環境変数

| 変数 | デフォルト | 説明 |
| --- | --- | --- |
| `PORT` | `8080` | Web サーバーの待受ポート |
| `CAROLINE_DATA_DIR` | `.` | Hub の SQLite、Hub key、alert file の保存先 |
| `CAROLINE_DB` | `$CAROLINE_DATA_DIR/caroline.db` | SQLite database のパス |
| `CAROLINE_HUB_KEY` | `$CAROLINE_DATA_DIR/hub.key` | Hub の Ed25519 private key |
| `CAROLINE_PUBLIC_URL` | — | Agent Enrollment URLを生成するための公開Hub URL |
| `CAROLINE_RETENTION` | `7d` | ログ保持期間。`0`、`off`、`disabled`で無効化 |
| `CAROLINE_MAX_STORAGE_SIZE` | `10GiB` | 保持ログpayloadの論理上限。`0`、`off`、`disabled`で無効化 |
| `CAROLINE_RETENTION_MODE` | `independent` | `independent`はHub基準、`source`はDocker側から報告されたログ境界、`min`は両者の短い方を使用 |
| `ALERTS_FILE` | `alerts.json` | アラートのJSON保存先 |
| `CAROLINE_HUB_URL` | — | Agent: Hub の URL |
| `CAROLINE_ENROLL_URL` | — | Agent: 単回利用のEnrollment URL。初回登録時にHub URLも保存 |
| `CAROLINE_ENROLLMENT_TOKEN` | — | Agent: 単回利用の登録 token |
| `CAROLINE_HUB_PUBLIC_KEY` | — | Agent: base64 raw Ed25519 Hub 公開鍵。TOFU を使わない限り必須 |
| `CAROLINE_AGENT_STATE_DIR` | `/var/lib/caroline-agent` | Agent identity、永続Hub pin（`hub.json`）、spool の保存先 |
| `CAROLINE_AGENT_SPOOL_MAX_SIZE` | `1GiB` | Agent spool 上限。超過時は古い batch から削除 |
| `CAROLINE_AGENT_SPOOL_MAX_AGE` | `24h` | Agent spool の最大保存期間 |
| `CAROLINE_AGENT_TRUST_ON_FIRST_USE` | `false` | 初回 Hub key を自動 pin。通常は公開鍵を明示 |
| `CAROLINE_AGENT_COMPRESSION` | `gzip` | Agent の batch 圧縮方式。`identity`、`gzip`、`zstd` |
| `DOCKER_HOST` | Docker のデフォルト | Agent が接続する Docker Engine |

`DOCKER_HOST` は `unix://`、`tcp://`、`http://`、`https://` の形式に対応しています。TCP / HTTP 接続を使う場合は、Docker Engine 側の認証・TLS・ネットワーク制御を別途設定してください。

Hub は Docker socket なしで起動できます。Hub mode の `/api/status` は `mode: "hub"` を返し、ログは認証済み Agent から到着します。単一ホストでも Hub に socket を渡さず、Hub と同じホスト上で Agent を起動する構成を推奨します。

Hubはデフォルトで7日より古いログを削除し、保持payloadの論理合計も10GiBに制限します。`CAROLINE_RETENTION_MODE=source`ではAgent/コンテナごとに報告されたDockerログの最古時刻より前を削除し、`min`ではHubの保持期間とsource境界の両方を適用します。Agentは`max-size`と`max-file`から保持時間を推定せず、Docker APIから取得した現在の最古ログ時刻を送信します。Docker側の境界を取得できない場合は推測で削除せず、Hub側のログを保持します。この容量は`text_payload`、`json_payload`、labels、summaryなどのログpayloadを対象とするもので、`caroline.db`全体のファイルサイズ上限ではありません。SQLiteのindex、row/page overhead、metadata table、WALなどが追加で必要です。cleanupは起動時と1時間ごとに実行します。Agentの未送信spool（デフォルト1GiB / 24時間）はHubの保存制限とは別です。

### Agent の登録

```sh
curl -X POST http://localhost:8080/api/nodes \
  -H 'Content-Type: application/json' \
  -d '{"ttlSeconds":900}'
```

APIレスポンスの`enrollmentUrl`を初回Agent起動時の`CAROLINE_ENROLL_URL`に設定します。URL登録時にHub challengeを検証し、Hub公開鍵とHub URLを`hub.json`へ保存します。以後はAgentの永続Ed25519 keyで署名し、再起動時に同じidentityとHubへ再接続します。既存構成では`CAROLINE_HUB_URL`、`CAROLINE_ENROLLMENT_TOKEN`、`CAROLINE_HUB_PUBLIC_KEY`も引き続き利用できます。node management API は、信頼できるネットワークの外へ公開する前に reverse proxy 等のアクセス制御で保護してください。

## UI の使い方

### Filters

- **Container**: 起動中コンテナを選択
- **Node**: Docker ホストを選択
- **Stream**: `stdout` または `stderr`
- **Severity**: `Errors`、`Warnings`、`Info`、`Debug` の完全一致
- **Time**: 5 分、15 分、1 時間、6 時間、24 時間、7 日、またはカスタム範囲
- **Search Logs**: 全フィールドを対象にしたテキスト検索

Container、Stream、Severity、Time のプリセット変更は即時に検索します。カスタム時間範囲は From / To を入力して **Apply** を押します。Sort の変更も即時に反映されます。Search Logs は入力停止後 300ms で検索します。

高度な条件は **Show Query** から編集します。Advanced Query は **Run Query**、またはクエリエディタ上で `Ctrl + Enter` / `Cmd + Enter` を押して実行します。Basic filters と Advanced Query は `AND` で結合されます。

**Reset Filters** は検索語、コンテナ、Stream、Severity、時間範囲、Advanced Query を初期状態へ戻します。

### Streaming

Streaming が有効なとき、初回検索後に `/api/tail` へ SSE 接続し、新着ログを画面へ追加します。5 秒間隔のポーリングではありません。

Streaming を停止すると SSE 接続を閉じます。フィルター、時間範囲、sort、クエリを変更した場合は、現在のストリームを閉じて新しい検索結果から再接続します。接続が切れた場合はブラウザの `EventSource` が再接続します。

### Alerts

現在のクエリから、しきい値、時間枠、クールダウン、severity、labels、任意の Runbook URL、サンプルの秘匿化モードを指定してアラートを作成できます。UI上の入力項目は `Runbook URL` と `Webhook URL` の2つだけで、Webhook URLのHTTPSホストとパスから送信先を自動判定します。Discord（`discord.com/api/webhooks/...`）、Slack Incoming Webhook（`hooks.slack.com/services/...`）、ntfy topic（`ntfy.sh/<topic>`）、Microsoft Teams Workflow（`*.logic.azure.com/.../paths/invoke` または `*.api.powerplatform.com/powerautomate/...`）に対応し、それ以外のURLには汎用JSON payloadを送信します。公開 URL を `CAROLINE_URL` に設定すると、通知に時間範囲付きの Explorer へのリンクを含めます。アラートエンジンは SSE と同じ共有 Docker `follow` ストリームを利用するため、複数のルールが同じコンテナを対象にしても Caroline 側の follow ストリームはコンテナごとに 1 本です。

Explorer の **Manage Alerts** から、アラートポリシーを独立した管理画面で確認できます。ポリシー数、発火中・有効・通知設定済みのサマリー、名前やクエリによる検索、状態フィルター、編集、一時停止・再開、削除に対応します。Webhook URL は一覧や API レスポンスに表示されず、既存の Webhook を保持したまま他の設定を編集できます。

ルールとアラート状態は `ALERTS_FILE` のJSONファイルへ保存され、起動時に復元されます。ログ本文や一致したエントリ自体は保存せず、時間枠の集計と通知に必要なタイムスタンプ、発火開始時刻、peak 件数、container 名などの小さなメタデータだけを保持します。状態が `OK` と `FIRING` の間で遷移し、Webhook を設定したルールでは発火・解消時に通知します。Docker Composeでは `caroline-data` volume に保存されます。

### Timeline / Fields / Logs

- **Timeline**: 表示幅に応じて検索結果を 24〜96 区間に分け、severity 別に表示します。バーのクリック、または範囲ドラッグで時間範囲を変更できます。ズームボタンも利用できます。
- **Fields**: 検索結果に含まれる System Metadata と Frequent Fields を集計します。フィールドの値をクリックすると、その値を Advanced Query に追加します。
- **Logs**: 新しい順 / 古い順の切り替え、Wrap Lines、Load More に対応します。
- 行を選択すると詳細 drawer が開き、Summary、Payload、メタデータ、Entry JSON のコピーを確認できます。

PC とモバイルでは、左上のメニューボタンからナビゲーションドロワーを開きます。「探索」「検出」カテゴリは折りたたみ可能で、実装済みの Logs、Timeline、Fields、Alerts へ移動できます。Fields はモバイルでは bottom sheet として表示されます。

### キーボード操作

- `/`: Search Logs にフォーカス
- `Ctrl + Enter` / `Cmd + Enter`: Advanced Query を実行
- `j` / `ArrowDown`: 次のログ行へ移動
- `k` / `ArrowUp`: 前のログ行へ移動
- `Home` / `End`: 先頭 / 末尾のログ行へ移動
- `Escape`: 詳細 drawer、モバイルナビ、メニューを閉じる

## Caroline Query Syntax

Cloud Logging に着想を得ていますが、完全互換ではありません。

```text
severity >= ERROR
resource.labels.container_name = "nginx"
stream = stderr
SEARCH("timeout")
jsonPayload.status >= 500
timestamp >= "2026-08-10T00:00:00Z"
```

### 演算子

対応する演算子は次のとおりです。

```text
=   !=   :   >   <   >=   <=
```

- `=` / `!=`: 大文字・小文字を区別しない比較
- `:`: 大文字・小文字を区別しない部分一致
- `severity` の比較演算子: severity rank による比較
- `timestamp` の比較演算子: RFC3339 timestamp として比較
- 改行: `AND` として扱う
- `AND` / `OR`: 句の結合

フィールドとして、`severity`、`stream`、`logName`、`resource.type`、`resource.labels.node_id`、`resource.labels.node_name`、`resource.labels.container_name`、`resource.labels.container_id`、`resource.labels.image`、`timestamp`、`labels.*`、`jsonPayload.*`、`textPayload` / `message` を利用できます。`node`、`container`、`container.name` などの短縮名にも対応しています。

`SEARCH("text")` は Summary、Text Payload、Log Name、Resource の種類・ラベル、JSON Payload を対象に検索します。`NOT`、括弧による優先順位、正規表現、Cloud Logging 固有の関数には対応していません。

## データモデルと制限

Docker のログ行は、Cloud Logging の LogEntry に近い形式へ正規化されます。

- `resource.type`: `docker_container`
- `resource.labels`: Node ID / name、コンテナ名 / ID、イメージ
- `logName`: `containers/<container>/<stream>`
- `severity`: ログ本文のキーワードから推定した `DEBUG` / `INFO` / `WARNING` / `ERROR`
- `stream`: `stdout` または `stderr`
- JSON として解釈できる行は `jsonPayload` も保持

severity は Docker が持つ標準属性ではありません。`ERROR`、`FATAL`、`PANIC`、`WARN`、`DEPRECATED`、`DEBUG`、`TRACE` などの本文キーワードから推定します。

安全に検索量を制限するため、次の上限があります。

- コンテナごとの取得対象: 最新 1,000 行
- 1 レスポンスの最大エントリ数: 50,000 件
- `/api/explorer` の 1 ページ: 最大 1,000 件（UI は通常 100 件ずつ取得）
- Docker Engine への検索リクエスト同時実行数: 8
- Streaming 対象コンテナ数: 8
- 1 ログ payload / follow frame の上限: 8 MiB

そのため、レスポンスの `approximate` は常に近似検索であることを示します。UI の `Partial` 表示から、コンテナごとの取得行数を確認できます。時間範囲を広げても、コンテナの過去ログを無制限に取得することはありません。

ページングには timestamp と insert ID を使った不透明な `nextPageToken` を使用します。次のページを取得するときは、同じ検索条件、時間範囲、sort を維持してください。

## API

読み取り API は GET に対応し、HEAD も受け付けます。アラート API はルールの作成・更新・削除のため POST、PUT / PATCH、DELETE も使用します。

### `GET /api/health`

Caroline サーバーの稼働確認を返します。

```json
{"ok":true,"service":"caroline"}
```

### `GET /api/status`

Hub mode または Docker Engine への接続状態、Docker version、API version、確認時刻を返します。Hub response では `mode: "hub"` が返され、`connected` は Docker の health signal ではありません。

### `GET /api/explorer`

検索結果の snapshot を返します。

主な query parameter:

| パラメータ | 説明 |
| --- | --- |
| `duration` | `5m`、`1h`、`7d`、`30d`、Go duration など。最大 30 日 |
| `from` / `to` | RFC3339 timestamp による範囲指定 |
| `q` | Caroline Query Syntax |
| `severity` | severity フィルター |
| `stream` | `stdout` / `stderr` |
| `containers` | コンテナ名、短縮 ID、完全 ID。カンマ区切り |
| `nodes` | Node ID または Node 名。カンマ区切り |
| `sort` | `asc` または `desc` |
| `limit` | 1〜1,000。デフォルト 100 |
| `pageToken` | 前レスポンスの `nextPageToken` |

例:

```sh
curl 'http://localhost:8080/api/explorer?duration=15m&limit=100&q=severity%20%3E%3D%20ERROR&sort=desc'
```

レスポンスには `entries` のほか、表示幅に応じた 24〜96 区間の `timeline`、`containers`、`fields`、`total`、`generatedAt`、`from`、`to`、`approximate`、`logTail`、`entryLimit`、`truncated` が含まれます。

### `GET /api/tail`

新着ログを Server-Sent Events で返します。

主な query parameter は `since`、`q`、`severity`、`stream`、`containers`、`nodes` です。`since` は RFC3339 timestamp で指定します。

イベント:

- `ready`: 接続時の対象コンテナ数と開始時刻
- `log`: 新しいログエントリ。data は `/api/explorer` の entry と同じ形式
- `warning`: Streaming 対象数の上限などの警告
- `error`: 特定コンテナの Streaming エラー
- `end`: 全ストリーム終了

接続中は 15 秒ごとに SSE keep-alive comment を送信します。

SSE エンドポイントとアラートエンジンは `logstream.Manager` を共有します。すでに Caroline が監視しているコンテナへブラウザが接続しても、Docker の follow ストリームを追加で作成しません。

### `/api/alerts`

アラートルールは `ALERTS_FILE` で指定したJSONファイルで管理します。`GET /api/alerts` で一覧、`POST /api/alerts` で作成、`GET` または `PUT /api/alerts/{id}` で全項目を更新、`PATCH /api/alerts/{id}` で指定項目だけを更新、`DELETE /api/alerts/{id}` で削除できます。PATCH で省略した項目は保持されます。Webhook URLを含むため、保存ファイルには `0600` の権限が設定されます。

作成例:

```json
{
  "name": "nginx errors",
  "query": "resource.labels.container_name = \"nginx\" AND severity >= ERROR",
  "severity": "critical",
  "labels": {
    "service": "nginx",
    "environment": "production"
  },
  "runbookUrl": "https://runbooks.example.test/nginx-errors",
  "sampleMode": "summary",
  "threshold": 5,
  "windowSeconds": 60,
  "cooldownSeconds": 600,
  "webhookUrl": "https://alerts.example.test/caroline"
}
```

Webhook payload には `alert.firing` または `alert.resolved`、stable な Rule ID、severity と labels、ルール Query、現在値と peak 値、しきい値、時間枠、container、発火開始時刻、通知時刻、Explorer URL、Runbook URL、秘匿化済みのサンプルエントリが含まれます。`sampleMode` は `off` / `summary` / `full` から選べ、full でも Caroline 外へ送信する前に秘匿化します。Webhook URL 自体は API のレスポンスに含めません。

Discord Incoming Webhook は Discord の [Execute Webhook](https://docs.discord.com/developers/resources/webhook#execute-webhook) 仕様に合わせ、`embeds` と `allowed_mentions: {"parse": []}` を含めて送信します。Discord 側の送信確認を得るため `wait=true` も付与します。Slackは公式の [Incoming Webhooks](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/) JSON / Block Kit形式、ntfyは公式の [公開ヘッダーと本文](https://docs.ntfy.sh/publish/)、TeamsはMicrosoftの [Teams webhook connector](https://learn.microsoft.com/en-us/connectors/teams/) のmessage envelopeとAdaptive Card形式で送信します。未知のホストには従来どおり汎用JSON payloadを送信します。

## 開発

```sh
npm ci
npm run typecheck
npm run build
go test ./...
```

### ディレクトリ

```text
.
├── cmd/caroline/        # Go のエントリーポイント
├── internal/docker/     # Docker Engine クライアントとログフレーム処理
├── internal/explorer/   # 正規化、検索、Timeline、フィルター
├── internal/logstream/  # 共有 Docker follow ストリームと購読
├── internal/alert/      # JSON永続化対応のアラートエンジンと Webhook 通知
├── internal/httpserver/ # HTTP API、SSE、アラート、静的ファイル配信
├── web/                 # フロントエンドアプリケーション
│   ├── index.html       # Vite のアプリケーションエントリ
│   ├── public/          # Vite がそのままコピーする任意の静的ファイル
│   └── src/
│       ├── app/         # bootstrap と URL state
│       ├── features/    # Explorer、filter、Timeline、log、streaming
│       ├── shared/      # API、DOM、format、i18n、型
│       ├── ui/          # JSX のアプリケーションシェル
│       └── styles/      # CSS layer
├── static/              # Vite のビルドで生成される配信物
├── Dockerfile
├── docker-compose.yml       # Hub
└── compose.agent.yml        # Agent
```

## セキュリティと運用上の注意

認証・認可機能はありません。Docker socket へのアクセス権を持つアプリとして動作するため、8080 番ポートは信頼できるローカルネットワーク内だけで公開してください。TCP の Docker Engine を使う場合は、Caroline の外側で TLS、認証、ファイアウォールを設定してください。

ログ本文や検索結果は Caroline に保存されません。各リクエストで Docker Engine から読み取り、ブラウザ内の状態として表示します。アラートの設定と集計用タイムスタンプは `ALERTS_FILE` に保存されます。

UI は IBM Plex Sans、ログ本文・timestamp・query editor・container ID・field 名などの等幅表示は IBM Plex Mono を使用します。Google Fonts が利用できない場合はシステムフォントへフォールバックします。Container Queries、`subgrid`、CSS Nesting、`dvh`、`light-dark()` などを使用しているため、比較的最新のブラウザで利用してください。
