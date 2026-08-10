# Caroline
<img width="1341" height="629" alt="top" src="https://github.com/user-attachments/assets/d89ad9db-64fa-45c4-93af-3e7329bdca6e" />

Caroline は、Docker Engine で現在起動しているコンテナの stdout / stderr を検索・閲覧するための軽量な Web アプリです。Google Cloud Logs Explorer の情報設計に着想を得ていますが、クエリ構文やデータモデルは Docker ログ向けの小さな subset です。

## 作った理由
自分のサーバでは主に全てコンテナ管理しているが統合したログ確認などが出来ず、外出先から確認するときは `Beszel` での単体のコンテナログを確認していました。ただしなかなかログ管理という観点だとちょっと厳しかったので個人的に使いやすい GCP の Cloud Logging の機能である `Logs Explorer` のような UI/UX で操作できればいいなと思い作成に至りました。  
アプリ名 / プロジェクト名 はたまたま The Velvet Underground の Caroline を聞いていただけでつけたもの。  

## 主な特徴

- 起動中コンテナを自動検出
- コンテナ、stdout / stderr、severity、時間範囲での絞り込み
- 全フィールド検索と Caroline Query Syntax による検索
- Timeline、Fields 集計、ログ詳細 drawer
- SSE による新着ログの Streaming 表示
- しきい値・時間枠・クールダウンに対応したログアラートと汎用 Webhook（Discord Incoming Webhook 対応）
- URL に検索条件を保存する Share Link
- ダーク / ライトテーマ、モバイル用ナビゲーション
- Docker Engine への読み取り専用アクセス

## 起動

### Docker Compose

```sh
docker compose up -d --build
```

ブラウザで <http://localhost:8080> を開きます。

Compose は `/var/run/docker.sock` を read-only でコンテナへマウントします。Caroline は Docker Engine の GET API のみを使用し、ログや検索結果を永続化しません。

停止する場合:

```sh
docker compose down
```

### ローカルで起動

必要なもの:

- Go 1.26 以上
- Node.js 22 以上と npm
- 接続可能な Docker Engine

```sh
npm ci
npm run build
go run ./cmd/caroline
```

デフォルトでは <http://localhost:8080> で待ち受けます。

## 環境変数

| 変数 | デフォルト | 説明 |
| --- | --- | --- |
| `PORT` | `8080` | Web サーバーの待受ポート |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker Engine の接続先 |
| `ALERTS_FILE` | `alerts.json` | アラートのJSON保存先 |

`DOCKER_HOST` は `unix://`、`tcp://`、`http://`、`https://` の形式に対応しています。TCP / HTTP 接続を使う場合は、Docker Engine 側の認証・TLS・ネットワーク制御を別途設定してください。

Docker socket を読めない場合でも Caroline 自体は起動しますが、画面には Docker 接続エラーが表示されます。

## UI の使い方

### Filters

- **Container**: 起動中コンテナを選択
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

現在のクエリから、しきい値、時間枠、クールダウン、任意の Webhook URL を指定してアラートを作成できます。Discord Incoming Webhook（`https://discord.com/api/webhooks/...`）を指定した場合は、Discord の `embeds` 形式で通知します。その他の URL には従来の汎用 JSON payload を送信します。アラートエンジンは SSE と同じ共有 Docker `follow` ストリームを利用するため、複数のルールが同じコンテナを対象にしても Caroline 側の follow ストリームはコンテナごとに 1 本です。

ルールとアラート状態は `ALERTS_FILE` のJSONファイルへ保存され、起動時に復元されます。ログ本文や一致したエントリ自体は保存せず、時間枠の集計に必要なタイムスタンプだけを保持します。状態が `OK` と `FIRING` の間で遷移し、Webhook を設定したルールでは発火・解消時に通知します。Docker Composeでは `caroline-data` volume に保存されます。

### Timeline / Fields / Logs

- **Timeline**: 表示幅に応じて検索結果を 24〜96 区間に分け、severity 別に表示します。バーのクリック、または範囲ドラッグで時間範囲を変更できます。ズームボタンも利用できます。
- **Fields**: 検索結果に含まれる System Metadata と Frequent Fields を集計します。フィールドの値をクリックすると、その値を Advanced Query に追加します。
- **Logs**: 新しい順 / 古い順の切り替え、Wrap Lines、Load More に対応します。
- 行を選択すると詳細 drawer が開き、Summary、Payload、メタデータ、Entry JSON のコピーを確認できます。

モバイルでは、左上のメニューボタンから Logs、Timeline、Fields を開きます。Fields は bottom sheet として表示されます。

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

フィールドとして、`severity`、`stream`、`logName`、`resource.type`、`resource.labels.container_name`、`resource.labels.container_id`、`resource.labels.image`、`timestamp`、`labels.*`、`jsonPayload.*`、`textPayload` / `message` を利用できます。`container`、`container.name` などの短縮名にも対応しています。

`SEARCH("text")` は Summary、Text Payload、Log Name、Resource の種類・ラベル、JSON Payload を対象に検索します。`NOT`、括弧による優先順位、正規表現、Cloud Logging 固有の関数には対応していません。

## データモデルと制限

Docker のログ行は、Cloud Logging の LogEntry に近い形式へ正規化されます。

- `resource.type`: `docker_container`
- `resource.labels`: コンテナ名、コンテナ ID、イメージ
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

Docker Engine への接続状態、Docker version、API version、確認時刻を返します。Docker が停止していても HTTP 200 の status payload を返し、`connected` が `false` になります。

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

主な query parameter は `since`、`q`、`severity`、`stream`、`containers` です。`since` は RFC3339 timestamp で指定します。

イベント:

- `ready`: 接続時の対象コンテナ数と開始時刻
- `log`: 新しいログエントリ。data は `/api/explorer` の entry と同じ形式
- `warning`: Streaming 対象数の上限などの警告
- `error`: 特定コンテナの Streaming エラー
- `end`: 全ストリーム終了

接続中は 15 秒ごとに SSE keep-alive comment を送信します。

SSE エンドポイントとアラートエンジンは `logstream.Manager` を共有します。すでに Caroline が監視しているコンテナへブラウザが接続しても、Docker の follow ストリームを追加で作成しません。

### `/api/alerts`

アラートルールは `ALERTS_FILE` で指定したJSONファイルで管理します。`GET /api/alerts` で一覧、`POST /api/alerts` で作成、`GET` または `PATCH` / `PUT /api/alerts/{id}` で取得・更新、`DELETE /api/alerts/{id}` で削除できます。Webhook URLを含むため、保存ファイルには `0600` の権限が設定されます。

作成例:

```json
{
  "name": "nginx errors",
  "query": "resource.labels.container_name = \"nginx\" AND severity >= ERROR",
  "threshold": 5,
  "windowSeconds": 60,
  "cooldownSeconds": 600,
  "webhookUrl": "https://alerts.example.test/caroline"
}
```

Webhook payload には `alert.firing` または `alert.resolved`、ルール名、現在の一致数、しきい値、時間枠、時刻、発火時のサンプルエントリが含まれます。Webhook URL 自体は API のレスポンスに含めません。

Discord Incoming Webhook は Discord の [Execute Webhook](https://docs.discord.com/developers/resources/webhook#execute-webhook) 仕様に合わせ、`embeds` と `allowed_mentions: {"parse": []}` を含めて送信します。Discord 側の送信確認を得るため `wait=true` も付与します。

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
└── docker-compose.yml
```

## セキュリティと運用上の注意

認証・認可機能はありません。Docker socket へのアクセス権を持つアプリとして動作するため、8080 番ポートは信頼できるローカルネットワーク内だけで公開してください。TCP の Docker Engine を使う場合は、Caroline の外側で TLS、認証、ファイアウォールを設定してください。

ログ本文や検索結果は Caroline に保存されません。各リクエストで Docker Engine から読み取り、ブラウザ内の状態として表示します。アラートの設定と集計用タイムスタンプは `ALERTS_FILE` に保存されます。

UI は IBM Plex Sans、ログ本文・timestamp・query editor・container ID・field 名などの等幅表示は IBM Plex Mono を使用します。Google Fonts が利用できない場合はシステムフォントへフォールバックします。Container Queries、`subgrid`、CSS Nesting、`dvh`、`light-dark()` などを使用しているため、比較的最新のブラウザで利用してください。
