**日本語** | [简体中文](README.zh-CN.md) | [English](README.en.md)

# Sleepy Go

「いま何してる？」をそのまま公開できる、軽量な個人用ステータスページです。手動で切り替えるステータス（オンライン / オフライン など）と、いま使っているデバイスやアプリを表示します。デバイスの情報は PC のクライアントや iOS のショートカットからリアルタイムに届きます。サーバーは Go で書かれていて、実行ファイル 1 つと SQLite ファイル 1 つだけで動きます。

## 主な機能

- **ステータス表示**：余白を活かしたシンプルなデザインで、状態のほか、使用中のデバイス・アプリ・バッテリー残量・再生中の曲と、見ている人の端末の時刻を表示します。デバイスの並び順は管理画面で変えられます。ライト / ダークモードはボタンで切り替えられます。
- **日本語・中国語・英語**：`page.language` で画面の言語を選べます（デフォルトは日本語）。
- **リアルタイム更新**：SSE で配信するので、ページを開いたままでも内容が更新されます。接続が切れても自動で再接続し、スマホでページに戻ればすぐ最新になります。
- **管理画面**：状態の切り替え、デバイスの管理、プライバシーモードに対応。表示する文言は、トップページのプレビューで文字をクリックするだけで編集できます（ランダム表示や変数も使えます）。
- **実行ファイル 1 つで完結**：ページ・CSS・JS・アイコン・タイムゾーンデータはすべて実行ファイルに組み込み済み。Python や Node、フロントエンドのビルドは必要ありません。
- **セキュリティ**：ドメインの許可リスト、デバイス専用シークレット、URL へのシークレット埋め込みの禁止に対応。リバースプロキシ（Caddy / nginx）の設定例も用意しています。
- **既存クライアントとの互換性**：`/api/…` に加えて、オリジナル版 Sleepy のクライアントが使う旧 API（`/device/set` など）にも対応しています。

## かんたんデプロイ（Docker）

Docker（Compose プラグイン入り）と git が使える Linux サーバーで、次のコマンドを実行します。

```sh
curl -fsSL https://raw.githubusercontent.com/kisukri/sleepy-go/main/deploy/install.sh | bash
```

リポジトリを `~/sleepy-go` に clone し、ランダムなシークレットを作ってからイメージをビルドし、`9010` 番ポートで起動します。最後にアクセス先の URL とシークレットが表示されます。

**オプション：独自ドメインと HTTPS**
スクリプトはリバースプロキシまではインストールしません。独自ドメインで HTTPS を使いたい場合は、Caddy や nginx を自分で用意してください（設定例は [`deploy/examples/`](deploy/examples/) にあります）。そのうえで、Sleepy をローカルだけで待ち受けさせ、そのドメイン宛てのアクセスだけを受け付けるようにします。

```sh
curl -fsSL https://raw.githubusercontent.com/kisukri/sleepy-go/main/deploy/install.sh | LISTEN=127.0.0.1 ALLOWED_HOSTS=status.example.com bash
```

インストール済みのサーバーでも、同じ変数を付けて `bash ~/sleepy-go/deploy/install.sh` を実行すれば切り替えられます。シークレットなどの設定はそのまま残ります。元に戻すときは `LISTEN=-`（外部公開に戻す）や `ALLOWED_HOSTS=-`（許可リストを外す）を指定してください。

| 変数 | デフォルト | 説明 |
| --- | --- | --- |
| `INSTALL_DIR` | `~/sleepy-go` | インストール先 |
| `PORT` | `9010` | Sleepy のポート番号 |
| `LISTEN` | すべてのインターフェース | `127.0.0.1` にするとローカルだけで待ち受けます（自前のリバースプロキシ用） |
| `ALLOWED_HOSTS` | なし | アクセスを許可するドメイン（カンマ区切り） |
| `SECRET` | 自動生成 | 管理画面へのログインとデバイスからの送信に使うシークレット |
| `PAGE_NAME` | `Sleepy` | ページに表示する名前 |
| `PAGE_LANGUAGE` | `ja` | 画面の言語（`ja` / `zh` / `en`）。オリジナル版から移行した場合は `zh` |
| `OLD_DIR` | `~/sleepy` | オリジナル版 Sleepy の場所（後述の「オリジナル版からの移行」を参照） |
| `REMOVE_OLD` | `1` | 移行に成功したら旧ディレクトリを削除します（事前にアーカイブを作成）。`0` なら残します |

**アップデート**は、同じスクリプトをもう一度実行するだけです。`git pull`、再ビルド、再起動をまとめて行います（数秒だけ止まります）。

```sh
bash ~/sleepy-go/deploy/install.sh
```

シークレットは `~/sleepy-go/docker-compose.yml`（パーミッション 600、Git の管理対象外）に、データは `~/sleepy-go/data/` に保存されます。

## おすすめのセキュリティ設定

1. リバースプロキシで HTTPS を有効にしましょう（例は `deploy/examples/`）。HTTPS なしだと、シークレットが暗号化されないままネットワークを流れます。
2. `data/config.yaml` で `main.device_secret` を設定し、クライアントにはこのデバイス用シークレットを使わせましょう。デバイス用シークレットでできるのはデバイス情報の送信だけです。`main.secret` は管理画面専用にします。
3. ショートカットがヘッダー（`Authorization: Bearer <デバイス用シークレット>`）で送るようになったら、`main.allow_query_secret: false` にしましょう。

詳しくは [設定 · セキュリティ](docs/configuration.md#セキュリティ) をご覧ください。

## オリジナル版 Sleepy からの移行

オリジナル版の [Sleepy](https://github.com/sleepy-project/sleepy)（Python）を Docker で動かしているサーバーなら、上のコマンドをそのまま実行するだけで移行できます。場所はデフォルトで `~/sleepy` で、違う場所なら `OLD_DIR=/path` で指定します。スクリプトは次の順に処理します。

1. 先に新バージョンを clone してビルドします。この間、旧バージョンは動いたままです。
2. 旧 `docker-compose.yml` から、シークレットや名前などの `sleepy_` 設定とポート番号を引き継ぎます。
3. 旧コンテナを止め、旧ディレクトリを丸ごと `~/sleepy-python-backup-<日時>.tgz` にまとめてから、`data/`（データベース・文言・設定）をコピーします。
4. Sleepy Go を起動してヘルスチェックを行います。失敗したら自動で旧バージョンに戻し、成功したら旧ディレクトリと旧イメージを削除します。

シークレット、デバイス ID、送信先はそのままなので、クライアント側の変更は不要です。管理画面だけは一度ログインし直してください。手動で移行する手順は [移行ガイド](docs/migration.md) にまとめています。

## その他の起動方法

### Docker Compose（手動）

```sh
git clone https://github.com/kisukri/sleepy-go.git && cd sleepy-go
mkdir -p data
export SLEEPY_UID="$(id -u)" SLEEPY_GID="$(id -g)"
export SLEEPY_MAIN_SECRET='長くてランダムなシークレットに置き換えてください'
export SLEEPY_PAGE_NAME='あなたの名前'
docker compose -f deploy/compose.yaml up --build -d
```

### ソースからビルド

**Go 1.25 以上**が必要です（`go.mod` でツールチェーン go1.27.3 を指定しているので、古い Go でも自動でダウンロードされます）。

```sh
git clone https://github.com/kisukri/sleepy-go.git && cd sleepy-go
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/sleepy-go ./cmd/sleepy-go
export sleepy_main_secret='長くてランダムなシークレットに置き換えてください'
export sleepy_page_name='あなたの名前'
./bin/sleepy-go
```

デフォルトでは `127.0.0.1:9010` で待ち受け、カレントディレクトリの `data/` にデータを保存します。外部に公開するには `sleepy_main_host=0.0.0.0` を設定してください。データの場所は `-data-dir /path/to/data` で変えられます。リバースプロキシや HTTPS については [デプロイと運用](docs/operations.md) を参照してください。

## 設定

ちょっとした設定なら、`sleepy_` で始まる環境変数（`sleepy_main_secret`、`sleepy_page_name` など）で済みます。細かく設定したい場合は、サンプルをコピーして編集してください。

```sh
cp configs/config.example.yaml data/config.yaml
```

優先順位は低い順に、デフォルト値 → `data/.env` / 環境変数 → `data/config.yaml` → `data/config.toml` → `data/config.json` です。シークレットは空にできず、短すぎるとログに警告が出ます。各項目の説明は [設定](docs/configuration.md) にあります。

## クライアントと API

状態を送るクライアントは [`clients/`](clients/README.md) にまとめています。Windows には Go 製のクライアント（exe 1 つで動作、Python 不要）があり、ほかに Python 版や PowerShell 版、Linux（KDE / Hyprland）向けのスクリプトもあります。iOS / iPadOS ではショートカットを使えます。サーバーのアドレス、シークレット、デバイス ID、表示名を設定すれば送信が始まります。

自分でクライアントを作る場合は、`Authorization: Bearer <シークレット>` を付けて `/api/device/set` に JSON を POST するのがおすすめです。詳しくは [API](docs/api.md) を参照してください。

以下の「シークレット」には、`main.device_secret` を設定していればそれを、なければ `main.secret`（管理画面のログインに使うもの）を使います。

### Windows クライアント

1. このリポジトリの [Actions の Go ワークフロー](https://github.com/kisukri/sleepy-go/actions/workflows/go.yml) で最新の成功した実行を開き、`sleepy-client-windows` をダウンロードします（GitHub へのログインが必要です）。展開したら、通常の PC は `sleepy-client.exe`、ARM 版 Windows は `sleepy-client-arm64.exe` を使います。
2. exe を決まったフォルダーに置いて、一度ダブルクリックします。同じフォルダーに `sleepy-client.toml` ができて、すぐに終了します。
3. `sleepy-client.toml` をメモ帳で開き、次の項目を書き込みます。

   ```toml
   server = "https://status.example.com"
   secret = "あなたのシークレット"
   device_id = "my-pc"
   device_name = "自分の PC"
   ```

4. もう一度ダブルクリックすると送信が始まります。画面は出ないので、動作は同じフォルダーの `sleepy-client.log` で確認できます。
5. 自動起動：`Win + R` で `shell:startup` を開き、exe のショートカットを置きます。

署名のない exe なので、初回は SmartScreen に止められることがあります。「詳細情報」→「実行」を選んでください。ほかの設定項目は [clients/](clients/README.md#sleepy-clientgo-版) にあります。

### iOS ショートカット

iOS 18 以上が必要です。「ショートカット」App で新しいショートカットを作り、次の順にアクションを追加します。

1. **前面のアプリを取得**
2. **バッテリー残量を取得**（残量を表示しない場合は省略できます）
3. **テキスト**：`[`、バッテリー残量、`%]`、アプリの「名前」をつなげます（例：`[87%]LINE`）。角かっこ内のパーセントはページで残量として表示されます。`[🔋87%]` でもよく、充電中は `[87%⚡]` と書けます。
4. **URL の内容を取得**：
   - URL：`https://status.example.com/api/device/set`
   - 方法：`POST`
   - ヘッダ：`Authorization`、値は `Bearer あなたのシークレット`
   - 本文を要求：JSON

   | キー | 種類 | 値 |
   | --- | --- | --- |
   | `id` | テキスト | `iphone`（デバイスごとに一意の ID） |
   | `show_name` | テキスト | `iPhone`（ページに出る名前） |
   | `using` | ブール | オン |
   | `status` | テキスト | 手順 3 の「テキスト」 |

一度手動で実行し、ページにデバイスが出れば成功です。あとは「オートメーション」→「App」ですべてのアプリを選び、「開いている」「すぐに実行」にしてこのショートカットを指定すると、アプリを開くたびに自動で送信されます。

アドレスは `https://` にしてください。`http://` ではシークレットが平文で流れるうえ、リバースプロキシが HTTPS へリダイレクトするときに POST が GET に変わったり、ヘッダーが落ちたりすることがあります。

## ディレクトリ構成

```text
.
├── cmd/sleepy-go/          # サーバーのエントリーポイント
├── cmd/sleepy-client/      # Windows 用 Go クライアント
├── internal/sleepy/        # 設定・HTTP API・認証・SQLite・SSE・文言とテスト
├── web/                    # 実行ファイルに組み込むページ・CSS・JS・文言エディター
├── clients/                # その他のクライアント（Python・PowerShell・シェル）
├── configs/                # 設定ファイルのサンプル
├── deploy/                 # インストールスクリプト・Dockerfile・Compose・リバースプロキシの例
└── docs/                   # 設定・API・運用・移行のドキュメント
```

## 開発

```sh
go test -race ./...
go vet ./...
```

## ライセンス

[MIT License](LICENSE) © Kisukri。`clients/` 内のスクリプトは [sleepy-project/sleepy](https://github.com/sleepy-project/sleepy) から引き継いだもので、元の作者による MIT ライセンス（[clients/LICENSE](clients/LICENSE)）が適用されます。オリジナル版の作者とクライアントの開発者の皆さんに感謝します。同梱の Newsreader フォントは SIL Open Font License 1.1 です（[web/static/fonts/OFL.txt](web/static/fonts/OFL.txt)）。
