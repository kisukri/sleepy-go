# API

サーバーのアドレスはデフォルトで `http://127.0.0.1:9010` です。レスポンスは UTF-8 の JSON で、エラー時は `success: false` と適切な HTTP ステータスコードを返します。リクエストボディは 1 MiB までで、10 秒以内に送り終える必要があります。受け付けるのは JSON オブジェクト 1 つだけです。`fields` の数値はそのまま保存されるので、2^53 を超える整数でも精度は落ちません。

## 認証

デバイスやスクリプトからのリクエストは、次のどれかの方法で認証します。

- `Authorization: Bearer <secret>`（おすすめ）
- `Sleepy-Secret: <secret>`
- JSON オブジェクト内の `secret` フィールド
- クエリパラメータの `secret`（旧クライアントのために残しています。新しく作るクライアントでは、URL にシークレットを入れないでください）。`main.allow_query_secret: false` で無効にできます。

`main.device_secret` を設定すると、そのシークレットはデバイス情報の送信（`/api/device/set`、`/device/set`）にしか使えなくなります。それ以外の書き込み API や管理画面へのログインには `main.secret` が必要で、デバイス用シークレットでは 403 になります。`main.allowed_hosts` を設定した場合、リストにない Host へのリクエストには 421 を返します。

ブラウザの管理画面は、ログイン後に `sleepy-session` という HttpOnly Cookie を使います。Cookie だけでは GET による書き込みはできず、管理画面からの書き込みは同一オリジンの POST に限られます。`/admin/data` は認証が必要で、公開側のプライバシーモードに関係なく、常にすべてのデバイス情報を返します。

## 状態とデバイス

| メソッド | パス | パラメータ / 動作 | 認証 |
| --- | --- | --- | --- |
| GET | `/api/meta` | バージョン、タイムゾーン、ページや表示の設定 | 不要 |
| GET | `/api/status/query` | 現在の状態、公開中のデバイス、更新時刻。`meta=true` を付けるとメタデータも返します | 不要 |
| GET | `/api/status/list` | 状態の一覧 | 不要 |
| GET | `/api/status/events` | SSE で状態を配信 | 不要 |
| POST / GET | `/api/status/set` | `status`：状態一覧のインデックス | 必要 |
| POST / GET | `/api/device/set` | デバイスの新規作成、または一部の更新 | 必要 |
| POST / GET | `/api/device/remove` | `id`：デバイス ID | 必要 |
| POST / GET | `/api/device/clear` | すべてのデバイスを削除 | 必要 |
| POST / GET | `/api/device/private` | `private=true/false`：公開側のプライバシーモード | 必要 |
| POST | `/api/device/order` | `{"order":["id1","id2"]}`：ページでのデバイスの並び順。並べていないデバイスはその後ろに、更新の新しい順で続きます。`main.secret` か管理画面のセッションが必要 | 必要 |
| GET | `/admin/data` | 管理用。状態、全デバイス、プライバシーモード、更新時刻 | 必要 |
| GET | `/none` | 204 を返すだけの死活監視用 | 不要 |

GET で書き込むには、シークレットを明示的に付ける必要があります（ブラウザの Cookie だけでは不可）。デバイスの JSON オブジェクト以外でも、POST では表にあるクエリパラメータをそのまま使えます。

デバイス情報の送信例：

```sh
curl -X POST http://127.0.0.1:9010/api/device/set \
  -H "Authorization: Bearer $sleepy_main_secret" \
  -H 'Content-Type: application/json' \
  -d '{"id":"my-pc","show_name":"My PC","using":true,"status":"Visual Studio Code","fields":{}}'
```

初めて登録するときは、空でない `id` と `show_name` が必要です。2 回目以降は変わった項目だけを送れば構いません。`status: ""` で前のタイトルを消せ、`using: false` もきちんと false として保存されます。省略した項目や null の項目は、元の値のまま残ります。

`using` には真偽値のほか、`"true"`/`"false"`/`1`/`0`/`yes`/`no` といった書き方も使えます（オリジナル版 Sleepy と同じ）。空、`none`、`null` は「変更しない」という意味になります。解釈できない値は、新しい API では 400 になりますが、V4 互換 API では旧クライアントの送信が失敗しないよう「変更しない」として扱います。`app_name` は `status` の互換用の別名で、`fields` は再帰的にマージされる拡張用のオブジェクトです。

クエリのレスポンス例：

```json
{
  "success": true,
  "time": 1700000000.125,
  "status": {"id": 0, "name": "活着", "desc": "目前在线", "color": "awake"},
  "device": {
    "my-pc": {
      "id": "my-pc",
      "show_name": "My PC",
      "using": true,
      "status": "Visual Studio Code",
      "fields": {},
      "last_updated": 1700000000.125
    }
  },
  "order": ["my-pc"],
  "last_updated": 1700000000.125
}
```

タイムスタンプは Unix 時間（秒）で、小数を含むことがあります。`order` は管理画面で決めたデバイスの並び順です（未設定なら空）。プライバシーモード中は公開側の `device` と `order` が空になりますが、送信は引き続き受け付けます。デバイスの `using` が null になっていることもあり、これはまだ使用状況が送られてきていないという意味です。

## SSE

`HEAD /api/status/events`（旧 `/events` も同様）はレスポンスヘッダーだけを返し、ストリームは開きません。`GET /api/status/events` は、接続した直後に完全な `update` イベントを 1 回送り、その後はデータが変わるたびに配信します。何も変化がなければ 30 秒ごとに `heartbeat` を送ります。切断後に再接続すると最新のスナップショットが届きます。`Last-Event-ID` を使った過去イベントの再送はしません。

```text
id: 1
event: update
data: {"success":true,...}

```

同時に維持できるストリーム接続は 256 本までで、それを超えると 503 になります。リバースプロキシではバッファリングを切り、アイドル接続のタイムアウトを十分に長くしてください。

## 管理画面と文言

| メソッド | パス | 動作 |
| --- | --- | --- |
| POST | `/admin/auth` | シークレットを確認してセッション Cookie を発行 |
| GET / POST | `/admin/verify` | 今のセッション、またはシークレットが有効か確認 |
| POST | `/admin/logout` | 管理画面の「ログアウト」ボタンが使います。同一オリジンからのリクエストならセッションの署名鍵を作り直し、ログイン中のすべてのブラウザを無効にしてからログイン画面に移ります |
| GET | `/admin/logout` | このブラウザのセッション Cookie だけを削除してログイン画面に移ります |
| GET | `/plugin/custom_text/text` | 公開用の文言設定と `more_text` |
| POST | `/plugin/custom_text/text` | `{"text": {...}}` を保存します（認証が必要） |

文言の API は `custom_text` が有効なときだけ使えます。グループは `page`、`time`、`status` の 3 つで、知らない項目は取り除かれ、文字数にも上限があります。編集には管理画面のエディターを使うのがおすすめです。保存したあと、トップページを再読み込みすると反映されます。

## V4 互換

`v4_compatible` はデフォルトで有効で、`/query`、`/events`、`/status_list`、`/set`、`/device/set`、`/device/remove`、`/device/clear`、`/device/private_mode`、`/save_data` を提供します。

旧 `/query` は、数値の状態 ID、`info`、デバイスの `app_name` を返し、時刻は設定したタイムゾーンで整形した文字列になります。新しい API は、状態オブジェクトと数値のタイムスタンプを返します。旧 `/save_data` はデフォルトでは deprecated のエラーを返し、`plugin.v4_compatible.simulate_save_data` を有効にしたときだけ以前の応答を再現します。

## エラーコード

- `400`：項目の型、状態のインデックス、JSON の形式が正しくない
- `401`：有効なシークレットまたはセッションがない
- `403`：Cookie を使った GET での書き込み、または別サイトからの管理画面の書き込み。レスポンスに受け取った Origin・Host・プロトコルが入るので、リバースプロキシの設定確認に使えます
- `404`：リソースが見つからない
- `408`：リクエストボディが 10 秒以内に届かなかった
- `413`：リクエストボディが大きすぎる
- `415`：空でない POST のボディが `application/json` ではない
- `421`：Host が `main.allowed_hosts` に含まれていない
- `500`：サーバー側のエラー（詳細はログに残ります）
- `503`：SSE のストリーム接続数が上限に達している

## 静的ファイル

ページが読み込むスクリプトとスタイルシートは、`/assets/<内容のハッシュ>/static/…` と `/assets/<内容のハッシュ>/editor/…` にあります。ハッシュはビルドの内容によって変わり、今のバージョンのファイルは長期間キャッシュ（immutable）されます。アップデートするとページが自動的に新しいアドレスを読みにいくので、手動でキャッシュを消す必要はありません。以前の `/static-themed/paper/<ファイル>` と `/static/<ファイル>` も引き続き使え、こちらは `cache_age` で指定した時間だけキャッシュされます。`/plugin/custom_text/editor.js|css` は廃止しました。
