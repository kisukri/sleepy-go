package web

// key -> [ja, zh, en]
var words = map[string][3]string{
	// page
	"title":        {"{name} のステータス", "{name} 的状态", "{name}'s status"},
	"updated":      {"更新：{updated_ago}", "{updated_ago}更新", "Updated {updated_ago}"},
	"live":         {"リアルタイム", "实时", "Live"},
	"connecting":   {"接続しています…", "连接中…", "Connecting…"},
	"reconnecting": {"接続が切れました。{n}秒後に再接続します", "已断开，{n} 秒后重连", "Disconnected. Retrying in {n}s"},
	"just_now":     {"たった今", "刚刚", "just now"},
	"minutes":      {"{n}分前", "{n} 分钟前", "{n} min ago"},
	"hours":        {"{n}時間前", "{n} 小时前", "{n} h ago"},
	"yesterday":    {"昨日", "昨天", "yesterday"},
	"days":         {"{n}日前", "{n} 天前", "{n} days ago"},
	"idle":         {"今は何も使っていません（最終アクティブ：{last_ago}）", "现在什么都没在用 · 上次活跃 {last_ago}", "Not using anything right now · last active {last_ago}"},
	"playing":      {"再生中", "播放中", "Playing"},
	"to_dark":      {"ダークモードにする", "切换到深色", "Switch to dark mode"},
	"to_light":     {"ライトモードにする", "切换到浅色", "Switch to light mode"},
	"c_green":      {"緑", "绿", "Green"},
	"c_grey":       {"グレー", "灰", "Grey"},
	"c_amber":      {"黄", "黄", "Amber"},
	"c_red":        {"赤", "红", "Red"},
	"c_blue":       {"青", "蓝", "Blue"},

	// login
	"login_title":   {"管理画面にログイン", "登录管理", "Sign in"},
	"login_check":   {"ログイン状態を確認しています…", "正在检查登录状态…", "Checking your session…"},
	"login_secret":  {"シークレットキー", "密钥", "Secret"},
	"login_ph":      {"サーバーに設定したシークレットキー", "输入服务器配置的密钥", "The secret set on the server"},
	"login_btn":     {"ログイン", "登录", "Sign in"},
	"login_back":    {"トップページに戻る", "返回首页", "Back to the status page"},
	"login_empty":   {"シークレットキーを入力してください", "请输入密钥", "Enter the secret"},
	"login_wrong":   {"シークレットキーが違います", "密钥不对，请重试", "That secret is wrong"},
	"login_failed":  {"ログインできませんでした（{n}）", "登录失败（{n}）", "Sign-in failed ({n})"},
	"login_offline": {"サーバーに接続できません。ネットワークを確認してください", "连不上服务器，请检查网络", "Can't reach the server. Check your connection"},

	// admin
	"admin":          {"管理画面", "管理", "Admin"},
	"admin_view":     {"サイトを表示", "查看首页", "View site"},
	"admin_logout":   {"ログアウト", "退出", "Sign out"},
	"admin_status":   {"現在のステータス", "当前状态", "Status"},
	"admin_devices":  {"デバイス", "设备", "Devices"},
	"admin_text":     {"表示テキスト", "文案", "Page text"},
	"loading":        {"読み込み中…", "加载中…", "Loading…"},
	"load_failed":    {"読み込めませんでした：{err}", "加载失败：{err}", "Couldn't load: {err}"},
	"dev_empty":      {"まだデバイスから受信していません", "还没有设备上报过", "No device has reported yet"},
	"dev_clear":      {"すべてのデバイスを削除", "清空所有设备", "Remove all devices"},
	"confirm_clear":  {"もう一度押すと、すべて削除します", "再点一次确认清空", "Tap again to remove all"},
	"cleared":        {"すべてのデバイスを削除しました", "已清空所有设备", "All devices removed"},
	"clear_failed":   {"削除できませんでした：{err}", "清空失败：{err}", "Couldn't remove: {err}"},
	"private":        {"プライバシーモード", "隐私模式", "Privacy mode"},
	"private_hint":   {"オンにすると、公開ページにデバイスを表示しません（受信は続けます）", "开启后首页不显示设备，上报照常进行", "Hides devices on the public page. Reports are still received"},
	"private_on":     {"プライバシーモードをオンにしました", "已开启隐私模式", "Privacy mode on"},
	"private_off":    {"プライバシーモードをオフにしました", "已关闭隐私模式", "Privacy mode off"},
	"expired":        {"ログインの有効期限が切れました", "登录已过期", "Your session has expired"},
	"switched":       {"「{name}」に切り替えました", "已切换为「{name}」", "Switched to “{name}”"},
	"switch_failed":  {"切り替えられませんでした：{err}", "切换失败：{err}", "Couldn't switch: {err}"},
	"in_use":         {"使用中", "在用", "In use"},
	"not_in_use":     {"未使用", "未在使用", "Not in use"},
	"more_actions":   {"{name} のその他の操作", "{name} 的更多操作", "More for {name}"},
	"device_id":      {"デバイス ID：", "设备 ID：", "Device ID: "},
	"dev_delete":     {"このデバイスを削除", "删除这台设备", "Remove this device"},
	"confirm_delete": {"もう一度押すと削除します", "再点一次确认删除", "Tap again to remove"},
	"deleted":        {"削除しました", "已删除", "Removed"},
	"delete_failed":  {"削除できませんでした：{err}", "删除失败：{err}", "Couldn't remove: {err}"},

	// editor: variables shown as 【label】
	"v_name":        {"名前", "名字", "name"},
	"v_status":      {"ステータス", "状态名", "status"},
	"v_desc":        {"説明文", "状态说明", "description"},
	"v_device":      {"デバイス名", "设备名", "device"},
	"v_app":         {"アプリ", "应用", "app"},
	"v_ago":         {"経過時間", "几分钟前", "time ago"},
	"v_count":       {"使用中の台数", "在用设备数", "devices in use"},
	"v_battery":     {"バッテリー", "电量", "battery"},
	"v_music":       {"再生中の曲", "正在播放", "music"},
	"v_last_ago":    {"最終アクティブ", "上次活跃", "last active"},
	"v_last_device": {"最後に使ったデバイス", "最后活跃设备", "last device"},
	"v_updated_ago": {"更新時刻", "更新于", "updated"},
	"v_time":        {"現在時刻", "当前时间", "current time"},
	"v_n":           {"数字", "数字", "number"},

	"e_name":       {"名前", "名字", "Name"},
	"e_title":      {"ステータス", "状态", "Status"},
	"e_desc":       {"説明文", "状态说明", "Description"},
	"e_device":     {"デバイスの行", "设备行", "Device rows"},
	"e_idle":       {"何も使っていないときの文", "都不在用时的文字", "Text when nothing is in use"},
	"e_foot":       {"フッター", "页脚", "Footer"},
	"e_footer":     {"フッターの追加テキスト", "页脚附加文字", "Extra footer text"},
	"e_edit":       {"{label}を編集", "修改{label}", "Edit {label}"},
	"e_sample_dev": {"サンプルデバイス", "示例设备", "Sample device"},
	"e_sample_app": {"サンプルアプリ", "示例应用", "Sample app"},

	"s_title":       {"「{status}」のときの表示", "「{status}」时的状态文字", "Status text while “{status}”"},
	"s_desc":        {"「{status}」のときの説明文", "「{status}」时的说明", "Description while “{status}”"},
	"s_idle":        {"「{status}」で、何も使っていないとき", "「{status}」时，设备都不在用", "While “{status}” with nothing in use"},
	"s_random":      {"複数書くと、ページを開くたびにランダムで1つ表示します", "写多句时，每次打开页面随机显示一句", "Write several lines and one is picked at random on each visit"},
	"s_dot":         {"丸の色", "圆点颜色", "Dot colour"},
	"s_emoji":       {"前に付ける絵文字（任意）", "前面的表情（可不填）", "Emoji in front (optional)"},
	"s_device_note": {"使用中のデバイスごとに1行表示します", "每台正在使用的设备显示一行", "One row for each device in use"},
	"f_device":      {"デバイス名", "设备名", "Device"},
	"f_app":         {"アプリ", "应用", "App"},
	"f_battery":     {"バッテリー残量を表示", "显示电量", "Show battery"},
	"f_music":       {"再生中の曲を表示", "显示正在播放", "Show music"},
	"f_under_app":   {"アプリ名の下に小さく表示します", "显示在应用名下方", "Shown under the app name"},
	"f_updated":     {"更新時刻", "更新时间", "Updated"},
	"f_live":        {"接続状態：接続済み", "实时提示：已连接", "Live indicator: connected"},
	"f_connecting":  {"接続状態：接続中", "实时提示：连接中", "Live indicator: connecting"},
	"f_reconnect":   {"接続状態：切断（【数字】は再接続までの秒数）", "实时提示：已断开（【数字】是倒计时秒数）", "Live indicator: disconnected (【number】 = seconds until retry)"},
	"s_footer_note": {"著作権表示などを入れられます。HTML のリンクも使えます。空欄なら設定ファイルの more_text を使います", "可放备案号，支持 HTML 链接；留空则使用配置里的 more_text", "For a copyright line or similar. HTML links work. Leave empty to use more_text from the config"},
	"s_pagetitle":   {"ブラウザのタブに表示するタイトル", "浏览器标签标题", "Browser tab title"},
	"s_time":        {"時間の表し方", "时间说法", "Time phrases"},
	"s_time_note":   {"【数字】は分・時間・日数に置き換わります", "【数字】会换成具体的分钟、小时或天数", "【number】 becomes the minutes, hours or days"},
	"t_just_now":    {"1分以内", "1 分钟内", "Under a minute"},
	"t_minutes":     {"1時間以内", "1 小时内", "Under an hour"},
	"t_hours":       {"24時間以内", "24 小时内", "Under a day"},
	"t_yesterday":   {"1日前", "1 天前", "One day ago"},
	"t_days":        {"それ以前", "更早", "Older"},
	"x_emoji_ph":    {"例：🌙", "例如 🌙", "e.g. 🌙"},
	"x_empty":       {"（空欄）", "（空）", "(empty)"},
	"x_insert":      {"挿入", "插入", "Insert"},
	"x_more_vars":   {"ほかの変数…", "更多…", "More…"},
	"x_del_line":    {"この文を削除", "删除这一句", "Remove this line"},
	"x_add_line":    {"+ 文を追加", "+ 再加一句", "+ Add a line"},
	"x_reset":       {"初期設定に戻す", "恢复默认", "Reset"},
	"x_cancel":      {"キャンセル", "取消", "Cancel"},
	"x_done":        {"完了", "完成", "Done"},
	"x_close":       {"閉じる", "关闭", "Close"},
	"x_saved":       {"保存しました。トップページを再読み込みすると反映されます", "已保存，首页刷新后生效", "Saved. Reload the status page to see it"},
	"x_save_failed": {"保存できませんでした：{err}", "保存失败：{err}", "Couldn't save: {err}"},
	"x_using":       {"使用中のデバイスあり", "有设备在用", "Devices in use"},
	"x_nothing":     {"何も使っていない", "都不在用", "Nothing in use"},
	"x_more":        {"その他", "更多", "More"},
	"x_live":        {"接続状態の表示", "实时提示", "Live indicator"},
	"x_shuffle":     {"ランダムの文を切り替える", "换一句随机文案", "Shuffle random lines"},
	"x_tip":         {"プレビューの点線で囲まれた文字を押すと編集できます", "点预览里带虚线框的文字即可修改", "Tap any dashed text in the preview to edit it"},
	"x_load_failed": {"テキストを読み込めませんでした：{err}", "文案加载失败：{err}", "Couldn't load the page text: {err}"},
}

// HTMLLang maps ja/zh/en to the html lang attribute.
func HTMLLang(lang string) string {
	switch lang {
	case "zh":
		return "zh-CN"
	case "en":
		return "en"
	}
	return "ja"
}

func Words(lang string) map[string]string {
	i := map[string]int{"zh": 1, "en": 2}[lang]
	out := make(map[string]string, len(words))
	for k, v := range words {
		out[k] = v[i]
	}
	return out
}
