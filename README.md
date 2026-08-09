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

フロントエンドのソースは `src/*.ts` に統一しています。`src/app.ts` はメインUI、`src/theme-init.ts` はCSS適用前に保存済みテーマを反映する独立エントリーポイントです。追跡対象のHTML/CSSは `public/` に置き、`npm run build` が `public/` を `static/` へコピーした後、TypeScriptを `static/` へ出力します。`static/` は生成物のためGit管理対象外です。実行時に使うのはコンパイル済みの標準ブラウザ JavaScript だけで、React などのUIランタイム依存はありません。

## Logs Explorer の構成

Google Cloud Logs Explorer の公開仕様と、ログイン済み Console を Chromium remote debugging 経由で観察した構成を参考にしています。Cloud Logging は Query pane、Fields pane、Timeline、Query results pane を同じ検索結果から更新し、`entries:list` のページングと `TailLogEntries` のストリーミングを分けています。

Caroline では Docker のログを Cloud Logging の `LogEntry` に近い形へ正規化し、次の構成で返します。

- `resource.type = "docker_container"` と container labels
- `logName`、`severity`、`timestamp`、`labels`、`textPayload` / `jsonPayload`
- Timeline の severity 別分布（Docker の tail 上での近似値）
- System Metadata / Frequent Fields の集計
- `AND` / `OR`、`=`, `!=`, `:`, `>=` などの Caroline Query Syntax（Cloud Logging に着想を得た subset。完全互換ではありません）

Docker Engine のログはコンテナごとに直近 1,000 行だけを取得します。時間範囲を広げても過去ログを無制限に読み込む仕様ではないため、Timeline と件数は「取得できた tail の範囲での近似値」です。1 回のレスポンスに含めるエントリ数にも 50,000 件の上限があり、超えた場合は `truncated: true` として返します。

複数コンテナを同時に調べる場合も、Docker Engine へのログ取得は最大 8 件ずつに制限します。取得結果は到着したものから集計するため、全コンテナの取得完了まで結果を別バッファへ溜め込みません。

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

`/api/explorer` の `nextPageToken` は、次のリクエストの `pageToken` にそのまま渡す不透明なカーソルです。現在の検索条件・時間範囲・sort を変えずに使うことで、ログが更新される間も offset 方式の重複や取りこぼしを避けて続きを取得できます。

Docker socket へのアクセス権がない環境では、アプリ自体は起動しますが、画面に接続エラーが表示されます。Compose では通常の Docker 環境で動作するよう socket をマウントしています。

認証機能はまだないため、8080 番ポートはローカルネットワーク内だけで公開してください。UI は IBM Plex Sans、ログやクエリなどの等幅表示は IBM Plex Mono を使用し、フォント取得時はシステムフォントへフォールバックします。
