# クライアント

デバイスの状態をサーバーに送るクライアントの一覧です。サーバー側の使い方は [README](../README.md) をご覧ください。

> [!NOTE]
> Windows 用の Go 製クライアント（`sleepy-client`）は [`cmd/sleepy-client/`](../cmd/sleepy-client/) にあります。それ以外のスクリプト（`windows/`、`linux/`）はオリジナル版のプロジェクトから引き継いだもので、元の作者の MIT ライセンス（[LICENSE](LICENSE)）が適用されます。
>
> このリポジトリには Windows / Linux / iOS（iPadOS）向けのクライアントだけを置いています。ほかのプラットフォーム向けは [オリジナル版](https://github.com/sleepy-project/sleepy/tree/main/client) にあり、送信 API は変わっていないのでそのまま使えます。

**Windows**
  - [sleepy-client（Go 版）](#sleepy-clientgo-版) *(おすすめ。exe 1 つで動き、Python は不要)*
  - [WinDevice](#windevice) *(Python 版。再生中の曲も送れます)*
  - [Win_Simple](#win_simple) *(画面付きの Python 版)*
  - [Win Fast Settings](#win-fast-settings使用停止)
  - [`Sleepy.Powershell.ps1`](./windows/Sleepy.Powershell.ps1) *(PowerShell スクリプト)*

**Linux**
  - [LinuxScriptKDE](#linuxscriptkde) *(KDE 用の Python スクリプト)*
  - [LinuxScriptHyprland](#linuxscripthyprland) *(Hyprland 用のシェルスクリプト)*
  - [NiriPiri](#niripiri) *(Niri 用のプラグイン、外部プロジェクト)*

**iOS / iPadOS**
  - [AppleShortcuts](#appleshortcuts) *(ショートカットの「前面のアプリを取得」アクションが必要)*

> [!TIP]
> どのクライアントも `/api/device/set` に送信します。旧アドレスの `/device/set` は、デフォルトで有効な内蔵モジュール `v4_compatible` が引き続き受け付けます。

# Windows

## sleepy-client（Go 版）

Windows で使っているウィンドウを自動でサーバーに送る、Go 製のクライアントです。exe ファイル 1 つで動き、Python や追加のライブラリは要りません。

送る内容：

- 前面にあるウィンドウのタイトル
- 一定時間マウスやキーボードの操作がなければ「未使用」
- ノート PC ならバッテリー残量と充電中かどうか（`[🔋82%⚡]` のようにタイトルの前に付きます）
- 定期的なハートビート（ウィンドウが変わらなくても数分おきに送るので、サーバーの `auto_dead` を安心して使えます）

再生中の曲には今のところ対応していません。必要な場合は [WinDevice](#windevice) のメディア情報機能を併用してください。

### 入手する

GitHub の Actions ページで最新の「Go」ワークフローを開き、成果物の `sleepy-client-windows` をダウンロードします（通常の PC は `sleepy-client.exe`、ARM 版 Windows は `sleepy-client-arm64.exe`）。

自分でビルドする場合は、リポジトリのルートで次のコマンドを実行します。

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H=windowsgui -s -w" -o sleepy-client.exe ./cmd/sleepy-client
```

### 設定

1. `sleepy-client.exe` を好きなフォルダーに置き、一度ダブルクリックします。同じフォルダーに設定ファイル `sleepy-client.toml` ができて、すぐに終了します。
2. `sleepy-client.toml` をメモ帳などで開き、`server`（サーバーのアドレス）、`secret`、`device_id`、`device_name` を書き込みます。`secret` には、サーバーで `main.device_secret` を設定していればそれを、なければ `main.secret` を使ってください。
3. もう一度ダブルクリックすると送信が始まります。画面には何も表示されないので、動作は同じフォルダーの `sleepy-client.log` で確認できます。

主な設定項目：

| 項目 | デフォルト | 説明 |
| --- | --- | --- |
| `interval_seconds` | `5` | 何秒ごとに確認するか |
| `heartbeat_seconds` | `300` | 変化がなくても何秒ごとに送り直すか |
| `idle_minutes` | `15` | 何分操作がなければ「未使用」にするか |
| `battery` | `true` | バッテリー残量をタイトルの前に付けるか |
| `app_first` | `false` | `ファイル名 - アプリ名` を `アプリ名 - ファイル名` の順に入れ替えるか |
| `app_only` | `false` | ウィンドウのタイトルではなくアプリ名（例：`Microsoft Edge`）だけを送るか。開いているタブやファイル名を公開したくないときに使います |
| `skip_titles` | `[]` | 無視するウィンドウタイトルの追加分（直前のタイトルを維持します） |
| `not_using_titles` | `[]` | 「未使用」とみなすウィンドウタイトルの追加分 |

タスクの切り替え画面や通知センターなど、一瞬だけ前面に出る Windows の画面は最初から無視します。ロック画面やスタートメニューは「未使用」として扱います。

### 自動起動

`Win + R` で `shell:startup` を開き、`sleepy-client.exe` のショートカットを置けば、ログイン時に自動で起動します。サインアウトやシャットダウンのときは「未使用」を送ってから終了します。

> [!TIP]
> 署名のない exe なので、初回は Windows の SmartScreen に止められることがあります。その場合は「詳細情報」→「実行」を選んでください。

## [WinDevice](./windows/win_device_ds.py)

> by: [@wyf9](https://github.com/wyf9) <br/>
> Co-authored-by: [@kmizmal](https://github.com/kmizmal) <br/>
> Co-authored-by: [@pwnInt](https://github.com/pwnInt) - **^C / マウス無操作の検出** <br/>
> Co-authored-by: [@gongfuture](https://github.com/gongfuture) - **メディア情報の取得** <br/>
> Co-authored-by: [@LeiSureLyYrsc](https://github.com/LeiSureLyYrsc) - **非同期処理** <br/>
> Co-authored-by: [@GoingScience](https://github.com/GoingScience) - **トレイからの起動**

Windows のデバイス状態を自動で送る Python 製のクライアントです。

必要なライブラリ：`httpx`、`pywin32`

### 設定

https://github.com/sleepy-project/sleepy/blob/23c14c9a8a32a29a6f60b3d4347e07a5e32e64fc/client/win_device.py#L29-L75

### ライブラリのインストール

```bat
:: Required. The others are optional (only for the matching feature)
pip install pywin32 httpx
```

```bat
:: Media info (Python <= 3.9)
:: winrt-runtime is for Python 3.10+ only (pulled in by the two winrt.windows.* packages below)
pip install winrt
```

```bat
:: Media info (Python >= 3.10)
pip install winrt.windows.media.control winrt.windows.foundation
```

```bat
:: Battery info
pip install psutil
```

```bat
:: Tray icon
pip install pystray Pillow
```

### 起動

```bat
python win_device.py
:: The tray icon only exists in the ds version
python win_device_ds.py
```

### 自動起動

方法は 2 つあります。

#### 1. PM2

PM2 を使うと、自動起動とプロセス管理ができます（参考：**[Windows PM2 自動起動](https://www.bing.com/search?q=Windows%20PM2%20startup)**）。

> PM2 での起動例：`pm2 start python --name sleepywin -- -u win_device.py` <br/>
> *ログが文字化けする場合は、文字コードの環境変数を設定してください。*

#### 2. 自動起動用スクリプト

`win_device_ds_autostart.vbs`

ウィンドウを表示せずに起動するためのスクリプトです。サードパーティのツールでプロセスを管理したくない場合に使えます。

1. `win_device_ds_autostart.vbs` を `shell:startup`（スタートメニューの「スタートアップ」）フォルダーに置く
2. `win_device_ds.py` を `%UserProfile%`（ユーザーフォルダー）に置く

> [!TIP]
> `shell:startup` と `%UserProfile%` は、`Win + R` の「ファイル名を指定して実行」から開けます。

### NetEase Cloud Music の曲情報が取れない場合

> **原因**：NetEase Cloud Music は SMTC に再生状態を渡さないため、曲情報を取得できません。

**対処法**：[BetterNCM](https://github.com/std-microblock/BetterNCM) をインストールし、`InfLink` プラグインの `SMTC` 機能を有効にすると取得できるようになります。

## Win Fast Settings（使用停止）

> by: [@Xiayue](https://github.com/tianyuzpr)

win_device.py をもとにした古い tkinter 製のツールで、参考として残しています。Go 版にはない metrics などの API を使っているため、今のバージョンの管理方法としてはおすすめしません。Web の管理画面を使ってください。

> [!WARNING]
> 最後に手が入ってから 1 年ほど経っており、最新の Sleepy で動くかどうかは作者も**保証していません**。
> このツールは **[WinDevice](#windevice) を前提にしています。使う前に win_device.py を同じフォルダーに置き、win_device.py の設定を済ませておいてください。**

追加で必要なライブラリ：`requests`

### 起動

```bat
python win_settings.py
```

## [Win_Simple](https://github.com/yangyan-ot/sleepy/releases/)

> by: [@kmizmal](https://github.com/kmizmal) <br/>
> Co-authored-by: [@yangyan-ot](https://github.com/yangyan-ot) - WinUI の画面とトレイ対応を追加 <br/>
> ソースコード：[`./windows/simple/script.py`](./windows/simple/script.py)

### 設定

設定ファイルは `config.ini` です（初回起動時に同じフォルダーに作られます）。WinUI の画面があるので、画面からそのまま設定できます。

> [!TIP]
> 自動起動したい場合は、`Win_Simple.exe` のショートカットを `C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Startup` に置いてください。

# Linux

## [LinuxScriptKDE](./linux/linux_device_kde.py)

> by: [@RikkaNaa](https://github.com/RikkaNaa)

Linux の KDE デスクトップ向けです。[kdotool](https://github.com/jinliu/kdotool) をインストールしておく必要があります。

ウィンドウ情報を取得できなかったときは「未使用」として扱います（[変数とタイミングの参考](https://github.com/RikkaNaa/sleepy/commit/9d5b4fc2014b725df24304beaa9439a5eb07099b)）。

### 設定

https://github.com/sleepy-project/sleepy/blob/7fc21380a259247533db76f3a0443fa550fcffec/client/linux_device_kde.py#L18-L28

### 使い方

自動起動はお好みの方法で設定してください。

> プロセスが `SIGTERM` を受け取ると、「未使用」を送ってから終了します。

## [LinuxScriptHyprland](./linux/linux_device_hyprland.sh)

> by: [@inoryxin](https://github.com/inoryxin)

Linux の Hyprland デスクトップ向けです。依存するものはなく、そのまま使えます。

### 設定

https://github.com/sleepy-project/sleepy/blob/7fc21380a259247533db76f3a0443fa550fcffec/client/linux_device_hyprland.sh#L7-L12

### 使い方

そのまま起動するだけです。

> [!TIP]
> 自動起動は `hyprland.conf` で設定できます。<br/>
> **スクリプトに実行権限（`chmod +x`）を付けておかないと動かないので注意してください。**

## [NiriPiri](https://github.com/RhenCloud/piri)

> by: [@RhenCloud](https://github.com/RhenCloud) <br/>
> ***外部のプロジェクトです***

**[piri](https://github.com/Asthestarsfalll/piri) をフォークしたものです。**

**piri の作者 [Asthestarsfalll](https://github.com/Asthestarsfalll) さんに感謝します。**

Linux の Niri デスクトップ向けです。piri の Sleepy プラグインが、ウィンドウのフォーカスが変わるたびにアプリの状態を自動で送ります。

### 設定

`~/.config/niri/piri.toml` でプラグインを有効にし、sleepy の設定を追加します。

```toml
[piri.plugins]
sleepy = true

[sleepy]
server_url = "https://sleepy.example.com"
device_id = "my-linux"
device_name = "Niri Desktop"
token = ""      # optional, Bearer token auth
secret = ""     # optional, some setups want secret in the body
prefer_app_id = false
```

> [!TIP]
> `server_url` にはサーバーのルートのアドレスを書くだけで、プラグインが `/api/device/set` に送信してくれます。

### 使い方

1. piri のデーモンをインストールして起動する
2. Niri のセッション中は piri を動かしたままにしておく
3. ウィンドウのフォーカスを切り替えると、Sleepy のデバイス状態が自動で更新される

関連ドキュメント：

- [original piri README](https://github.com/Asthestarsfalll/piri/blob/main/README.md)
- [piri README](https://github.com/RhenCloud/piri/blob/main/README.md)
- [Sleepy プラグインの説明（中国語）](https://github.com/RhenCloud/piri/blob/main/docs/zh/plugins/sleepy.md)

# iOS / iPadOS

## [AppleShortcuts](https://github.com/Detritalw/Sleepy-Client-Shortcuts)

> by: [@Detritalw](https://github.com/Detritalw) <br/>
> ***外部のプロジェクトです***

### FullVer

[完全版をインストール（Apple Watch・iPhone・iPad・Mac などに対応）](https://www.icloud.com/shortcuts/aa31f2a5295842939be354285d4e9d14)

### FastVer

[高速版をインストール](https://www.icloud.com/shortcuts/eec863215bfb4d7ea7228b6032d1fc6c)

**おすすめは、「オートメーション」→「App」→「開いている」でアプリをすべて選び、「すぐに実行」にしたうえで、ショートカットに「Sleepy Client Shortcuts Fast」を指定する使い方です。アプリを開くたびに自動で送信されるので、とても快適です。**

> [!WARNING]
> **iOS 18 以上が必要です**（「前面のアプリを取得」アクションを使うため）。
> ここのリンクは最新ではない可能性があります。[プロジェクト側の最新版も確認してください](https://www.icloud.com/shortcuts/92fbddbf922343f5b076c828f788371f)。

> [!TIP]
> このショートカットは、アクションボタン、コントロールセンター、ロック画面のボタン、背面タップ（ダブルタップ / トリプルタップ）に割り当てるとすぐに使えます。
