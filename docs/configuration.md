# 設定

コマンドはすべてリポジトリのルートで実行します。[`configs/config.example.yaml`](../configs/config.example.yaml) を `data/config.yaml` にコピーするか、`sleepy_` で始まる環境変数で設定してください。

## 読み込みの順番

```text
デフォルト値 < data/.env < システムの環境変数 < data/config.yaml < data/config.toml < data/config.json
```

`-data-dir` を指定すると、上の設定ファイル、デフォルトの SQLite データベース、`text.json`、公開ファイルの場所がまとめて変わります。設定を変えたら、サービスを再起動してください。環境変数の名前は大文字・小文字を区別せず、階層はアンダースコアでつなぎます（例：`sleepy_main_port`）。環境変数の型変換は設定ファイルとマージしたあとに行うので、設定ファイル側で上書きされる変数であれば、書式が間違っていても起動は止まりません。上書きされない不正な値はエラーになります。

データディレクトリには `session.key` も保存されます（管理画面のセッション署名に使うランダムなデータ。初回起動時に自動で作られ、パーミッションは 0600）。このファイルを消すと、管理画面のセッションがすべて無効になります。`main.secret` が 16 文字未満だと、起動時のログに警告が出ます。ログインの試行回数は制限していないので、十分に長いランダムなシークレットを使ってください。

## よく使う項目

| 設定項目 | 環境変数 | デフォルト / 説明 |
| --- | --- | --- |
| `main.secret` | `sleepy_main_secret` | 必須。空にはできません |
| `main.host` | `sleepy_main_host` | `127.0.0.1`。Docker イメージでは `0.0.0.0` |
| `main.port` | `sleepy_main_port` | `9010` |
| `main.timezone` | `sleepy_main_timezone` | `Asia/Shanghai`。旧 API（`/query`）の時刻に使います。ページの時計は閲覧者の端末の時刻です |
| `main.database` | `sleepy_main_database` | データディレクトリ内の `data.db`。SQLite のみ対応 |
| `main.cache_age` | `sleepy_main_cache_age` | `1200` 秒。フロントエンドのファイルをキャッシュする時間 |
| `main.cors_origins` | `sleepy_main_cors_origins` | `*`。オリジンのリストも指定できます |
| `page.name` | `sleepy_page_name` | `User` |
| `page.title` | `sleepy_page_title` | `User Alive?`。表示は管理画面で上書きできます |
| `page.desc` | `sleepy_page_desc` | ページの説明文 |
| `page.favicon` | `sleepy_page_favicon` | `/favicon.ico` |
| `page.more_text` | `sleepy_page_more_text` | フッターの HTML。信頼できる管理者が書いた内容だけを入れてください |
| `page.theme` | `sleepy_page_theme` | `paper` のみ |
| `page.language` | `sleepy_page_language` | `ja`。`ja` / `zh` / `en` から選びます。画面の文言と、デフォルトの状態名・`not_using` の言語が変わります |
| `status.device_slice` | `sleepy_status_device_slice` | `50`。アプリ名を表示する文字数。`0` で切り詰めなし |
| `status.refresh_interval` | `sleepy_status_refresh_interval` | `5000` ミリ秒。SSE を使えないブラウザでのポーリング間隔 |
| `status.not_using` | `sleepy_status_not_using` | `page.language` に合わせて `退席中` / `未在使用` / `Idle`。API で未使用のデバイスに付ける文字 |
| `status.status_list` | `sleepy_status_status_list` | 状態の一覧。並び順がそのまま ID になります |
| `plugins_enabled` | `sleepy_plugins_enabled` | デフォルトは `["v4_compatible", "custom_text"]` |
| `plugin` | `sleepy_plugin` | 内蔵モジュールの設定 |

環境変数でリストやオブジェクトを渡すときは、JSON 文字列で書きます。

```sh
export sleepy_plugins_enabled='["v4_compatible","custom_text"]'
export sleepy_main_cors_origins='["https://status.example.com"]'
```

`status.sorted` と `status.using_first` は、クライアントが参照できるようメタデータに含めていますが、paper の画面では使用中のデバイスを更新の新しい順に並べます。

## 状態の一覧

```yaml
status:
  status_list:
    - name: オンライン
      desc: いつでも気軽に連絡してください。
      color: awake
    - name: 退席中
      desc: 少し席を外しています。すぐ戻ります。
      color: sleeping
```

`status_list` を書かなければ、`page.language` に合わせたデフォルト（日本語なら「オンライン」「オフライン」、中国語なら「活着」「似了」）を使います。一覧には少なくとも 1 つの状態が必要で、ID は `0` から順に自動で振られます。paper での表示文言・色・絵文字は状態ごとに管理画面で変えられ、クライアントが使う状態 ID には影響しません。

## 表示言語

`page.language` で、トップページ・管理画面・ログイン画面の言語を切り替えます（`ja` / `zh` / `en`）。管理画面のエディターで保存した文言はこの設定より優先されるので、言語を変えても、自分で書いた文言はそのまま残ります。

ページ右上のボタンでライト / ダークモードを切り替えられます。選んだモードはブラウザに保存され、選ばなければ OS の設定に従います。

## 内蔵モジュール

- `custom_text`：トップページの文言と、プレビュー付きのエディター。データディレクトリの `text.json` に保存します。
- `v4_compatible`：旧クライアント向けに、`/api` が付かない API を提供します。`plugin.v4_compatible.simulate_save_data: true` で、旧 `/save_data` の応答を再現できます。
- `auto_dead`：任意。デフォルトは無効です。一定時間更新がないと全体の状態を指定した状態に切り替え、その後デバイスから送信があれば状態 `0` に戻します。

```yaml
plugins_enabled:
  - v4_compatible
  - custom_text
  - auto_dead
plugin:
  auto_dead:
    check_interval: 60
    timeout_minutes: 15
    dead_status_id: 1
```

`auto_dead` を有効にする前に、クライアントが定期的にハートビートを送るか確認してください（Go 版の `sleepy-client` は送ります）。古いクライアントの中には、ウィンドウが変わらないと送信しないものがあり、最後の送信時刻だけではデバイスがまだ使われているか判断できません。プライバシーモードは公開側のデバイス情報を隠すだけで、送信の受け付けやタイムアウトの判定は止めません。

## セキュリティ

| 設定項目 | 環境変数 | 説明 |
| --- | --- | --- |
| `main.device_secret` | `sleepy_main_device_secret` | デバイス専用のシークレット。`/api/device/set` と旧 `/device/set` にしか使えず、状態の変更、デバイスの削除、管理画面へのログインはできません。空のときはデバイスも `main.secret` を使います。`main.secret` と同じ値にはできません |
| `main.allow_query_secret` | `sleepy_main_allow_query_secret` | デフォルトは `true`。`false` にすると URL の `?secret=` を受け付けなくなり、ヘッダーか JSON に入れたシークレットだけが使えます |
| `main.allowed_hosts` | `sleepy_main_allowed_hosts` | アクセスを許可するドメインのリスト（環境変数ではカンマ区切りでも書けます）。設定すると、それ以外の Host（IP アドレスで直接アクセスした場合など）には 421 を返します。死活監視用の `/none` は対象外です |

おすすめの組み合わせは次のとおりです。

1. 任意：自分で用意した Caddy や nginx で HTTPS を提供し（例は `deploy/examples/`）、インストールスクリプトを `LISTEN=127.0.0.1 ALLOWED_HOSTS=あなたのドメイン` を付けて実行します。9010 番ポートはローカルでだけ待ち受け、そのドメイン宛てのアクセスだけを受け付けるようになります。
2. `device_secret` を設定し、クライアントやショートカットのシークレットをこれに差し替えます。`main.secret` は管理画面専用にします。
3. ショートカットが `Authorization: Bearer <device_secret>` ヘッダーで送るようになったら、`allow_query_secret` を `false` にします。旧 API を使うクライアントがなくなったら、`plugins_enabled` から `v4_compatible` を外しても構いません。

すべてのレスポンスに `X-Frame-Options: DENY` と `frame-ancestors 'none'` を付けて、ほかのサイトに埋め込まれないようにしています。HTTPS でアクセスされたときは HSTS も付けます。

## HTTPS とリバースプロキシ

リバースプロキシは必須ではなく、インストールスクリプトも自動では入れません。[`deploy/examples/`](../deploy/examples/) に、Caddy（証明書を自動で取得）と nginx（certbot と組み合わせて使用）の設定例があります。リバースプロキシを使うときは `LISTEN=127.0.0.1 ALLOWED_HOSTS=status.example.com bash ~/sleepy-go/deploy/install.sh` を実行してください。今のシークレットや設定はそのまま残ります。

`main.https: true`、`main.ssl_cert`、`main.ssl_key` を設定すれば、Sleepy が直接 HTTPS を提供することもできます。証明書のパスは作業ディレクトリからの相対パスで、デフォルトはそれぞれ `data/cert.pem`、`data/key.pem` です。

リバースプロキシを使う場合は、Host ヘッダーをそのまま渡し、`X-Forwarded-Proto` を正しく設定し、SSE のバッファリングを無効にしたうえで、30 秒間隔のハートビートより長く接続を保てるようにしてください。バックエンドに直接つなげるのは、信頼できるプロキシだけにしてください。HTTPS では、管理画面のセッション Cookie に Secure 属性が付きます。

## 公開ファイル

`data/public/`（またはデータディレクトリ内の `public/`）に置いたファイルは、同じ名前の URL で公開されます。たとえば `data/public/avatar.png` は `/avatar.png` で見られます。作業ディレクトリの `public/` より優先され、同じ名前のファイルを置けば内蔵の favicon も差し替えられます。ディレクトリの外を指すパスや、シンボリックリンクで外に出ることはできません。

データベース、シークレット、設定ファイル、バックアップは `public/` に置かないでください。`page.more_text` と管理画面のフッター文言には HTML を書けるので、信頼できる管理者だけが編集するようにしてください。
