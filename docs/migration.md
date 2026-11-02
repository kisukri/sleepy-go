# オリジナル版 Sleepy からの移行

Sleepy Go は、オリジナル版 [Sleepy](https://github.com/sleepy-project/sleepy)（Python）の SQLite データベース（`main`、`device_status` テーブル）と `text.json` をそのまま使えます。クライアントが使うシークレット、デバイス ID、送信用の API も変わりません。ただし、Python のプラグインは読み込まず、Jinja のテーマにも対応していません。ページのデザインは paper だけです。

## 自動で移行する（おすすめ）

オリジナル版を Docker Compose で `~/sleepy` に置いている場合は、サーバーで次のコマンドを実行します。

```sh
curl -fsSL https://raw.githubusercontent.com/kisukri/sleepy-go/main/deploy/install.sh | bash
```

別の場所に置いているなら `OLD_DIR` で指定し、旧ディレクトリを残したいなら `REMOVE_OLD=0` を付けます。

```sh
curl -fsSL https://raw.githubusercontent.com/kisukri/sleepy-go/main/deploy/install.sh | OLD_DIR=/srv/sleepy REMOVE_OLD=0 bash
```

スクリプトは次の順番で処理します。

1. 旧 `data/config.*` で、Go 版が対応していないテーマやプラグインを使っていないか確認します。使っていればその場で止めて知らせ、何も変更しません。
2. Sleepy Go を `~/sleepy-go` に clone してイメージをビルドします。その間、旧バージョンは動いたままです。
3. 旧 `docker-compose.yml` から `sleepy_` の環境変数（シークレット、名前など）とポート番号を読み取り、新しい `~/sleepy-go/docker-compose.yml`（パーミッション 600、Git の管理対象外）に書き込みます。コンテナは旧 `data/` の所有者として動くので、root が所有している旧データもそのまま使えます。
4. 旧コンテナを止め、旧ディレクトリを丸ごと `~/sleepy-python-backup-<日時>.tgz` にまとめてから、`data/` を新しいディレクトリにコピーします。
5. Sleepy Go を起動して `/none` で動作を確認します。うまくいかなかった場合は、新バージョンを止めて旧バージョンを起動し直し、作成した設定を `docker-compose.yml.failed` として残します。原因を直してから、もう一度実行してください。
6. 成功したら、旧コンテナのローカルイメージと旧ディレクトリを削除します（`REMOVE_OLD=0` のときは残します）。

移行したあとは、管理画面にもう一度ログインしてください（Cookie の形式が変わったため）。以降のアップデートは `bash ~/sleepy-go/deploy/install.sh` で行えます。

## 手動で移行する

1. オリジナル版のサービスを止め、データベースに書き込んでいるプロセスが残っていないことを確認します。
2. `data/` を丸ごとバックアップします（SQLite の `-wal`、`-shm` ファイルがあれば一緒に）。
3. バックアップのコピーを使って、試しに動かしてみます。

   ```sh
   export sleepy_main_secret='元のシークレット'
   ./bin/sleepy-go -data-dir /path/to/data-copy
   ```

4. 状態の一覧、デバイス、プライバシーモード、トップページの文言、クライアントからの送信に問題がないか確認してから、本番のデータディレクトリで動かします。

Docker を使う場合の注意点：イメージは `deploy/docker/Dockerfile` でビルドし、デフォルトでは uid 65532 として動きます。データディレクトリはコンテナの `/sleepy/data` にマウントします。データが root の所有になっている場合は、Compose に `user: "0:0"` を追加するか、`chown -R 65532:65532 data` を実行してください。そうしないと、起動時に「ディレクトリに書き込めない」というメッセージが出ます。

## 設定の違い

- `main.secret` は空にできません。
- `page.theme` は `paper` だけです。
- `plugins_enabled` に書けるのは `custom_text`、`v4_compatible`、`auto_dead` だけで、それ以外のプラグイン名があると起動しません。
- オリジナル版のログ、デバッグ、統計（metrics）の設定は効きません（`sleepy_main_colorful_log` などは無視されます）。
- 設定ファイルが環境変数を上書きする点は同じです。読み込みの順番は [設定](configuration.md) をご覧ください。

## 動作の違い

- 管理画面は署名付きの `sleepy-session` Cookie を使い、オリジナル版の `sleepy-secret` Cookie は受け付けません。
- ブラウザの Cookie で認証した書き込みは、同一オリジンの POST に限られます。クライアントからの GET による書き込みは、シークレットを明示的に付ければ使えます。
- `status: ""` でデバイスのタイトルを消せます。`using: false` が「省略」と誤って扱われることもありません。空の `using` は「変更しない」という意味です。
- 範囲外の状態 ID には 400、内部エラーには 500 を返し、HTTP 200 でごまかすことはありません。
- SSE は変更の通知で動きます。再接続したときは最新の状態を送り、過去の分は再送しません。
- `auto_dead` はデフォルトで無効です。有効にする前に、クライアントが定期的に送信することを確認してください。そうしないと、ウィンドウが変わらないだけのデバイスがオフラインと判定されてしまいます。

2 つのサービスから同じデータベースに同時に書き込まないでください。複数の Sleepy Go で 1 つのデータベースを共有することもできません。サービスはプロセス内に状態のスナップショットを持っていて、プロセス間では同期しないためです。

## 元に戻す

Sleepy Go を止め、移行時のバックアップ（自動で移行した場合は `~/sleepy-python-backup-<日時>.tgz`）からオリジナル版のディレクトリを復元して、元の方法で起動します。Sleepy Go はデータベースの既存のテーブルを削除しませんが、内容を確認していない新しいデータを旧バージョンにそのまま引き継がせるのは避け、バックアップから戻すことをおすすめします。
