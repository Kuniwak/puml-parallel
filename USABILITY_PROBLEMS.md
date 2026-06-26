# csdfreplcmd ユーザビリティ調査メモ

エージェントに `csdfreplcmd` で仕様アニメーション（`examples/valid/vending_machine.puml` の状態機械を対話的に歩く）をさせ、打ち間違え・思った通りにならなかった箇所を収集したもの。

## 調査方法

- 使い方を事前に教え込んでいない新規エージェント1体に、`csdfreplcmd help` と `README.md` だけを頼りに自動販売機仕様のエンドツーエンドシナリオ（start → insert → showPurchasable → choose → drop → idle、さらに途中から `jump` で分岐）を歩かせた。
- エージェントが報告したつまずきを、こちらで実機（`csdfrepld` 起動済み、socket は temp dir フォールバック）で1件ずつ再現し、コマンド文字列・出力・exit code・発生箇所を確認した。本メモの evidence はすべて再現確認済み。
- daemon はセッション削除済みでクリーンな状態に戻してある。

シナリオ自体は最後まで完走できた。ツールは「動く」が、エラーメッセージとヘルプの作り込みが弱く、エージェントが何度も `read` / `help` / README / ソースに戻って確認させられた。

---

## 対応状況（このセッションで実施）

下記は本ブランチ（`csdfreplcmd-help`）で TDD + Tidy First により修正済み。実機（リビルドした `csdfrepld`/`csdfreplcmd`）で回帰確認済み。

| 項目 | 対応 | 結果（既定出力） |
|---|---|---|
| **H-1** 内部識別子の漏れ | ✅ 修正 | `csdf.SolveJSON:` 等が消え root cause のみ表示。`csdfrepld -debug` でフルチェーン。エラー印字を `tools.NewCommandFunc`（`cli` から移設）に集約し `CommonOptions.LogLevel` で最深 Unwrap を一括判定 |
| **M-2** 個数不一致／値プロンプト | ✅ 修正 | `expected 1 value(s) for [availableProducts], got 2`。プロンプトも `Enter 1 value as a JSON array in declaration order: [<availableProducts>].` |
| **M-5** グループ `-h`/`help` | ✅ 修正 | `session -h` / `session help` / `help session` が詳細ヘルプを出して exit 0 |
| **L-1** `help <sub>` ＋ 詳細化 | ✅ 修正 | `help <sub>` が当該コマンドの `-h` に委譲。各リーフ `-h` に synopsis・説明・実例を付与（Coding Agent 向け） |
| **L-2** ヘルプ不統一／未発見時 | ✅ 修正 | 共有 `WriteCommandHelp` に統一。未知コマンドは整形ヘルプを出して exit 1（`no such subcommand` の漏れ除去） |
| **L-3** jump の分岐挙動 | ✅ 説明追加 | `jump -h` に「履歴を複製して末尾に追記し分岐（巻き戻し／切り詰めはしない）」を明記（コードは現状維持） |
| **L-4** `session list` 見出し | ✅ 修正 | `encoding/csv`（タブ区切り）で `ID/MODE/STATE/PATH` 見出し付き |
| **L-5** `name (id)` 冗長 | ✅ 修正 | 表示は name のみ（`State: vmIdle`）。id は `-json` 構造化出力に残置 |
| **L-6** 文言の大小不統一 | ✅ 触れた範囲で統一 | 新規・改修した文言は小文字始まりに（`Index out of range` 等の既存は M-3 が意図通りのため未変更） |
| **H-2** `session rm` 無印削除 | ⏸ 意図通り（変更なし） | 単一セッション時の `-s` 省略は仕様 |
| **M-1** index 後のフラグ誤認 | ⏸ 修正不要 | — |
| **M-3** 範囲外で有効範囲非表示 | ⏸ 意図通り（変更なし） | `Index out of range` のまま |
| **M-4** 負 index がフラグ扱い | ⏸ 修正不要 | — |

副次対応: 共有の `ValidateArgsAsFilePath` とクライアントのファイル読込エラーも、最深 Unwrap でファイル名や接頭辞が損なわれないようクリーンなリーフ文言に整理。クライアント leaf が `-debug/-silent/-v/-version` を受けるよう統一（top-level `-version` も復活）。

既知の積み残し: `csdf` パーサのエラーは `csdf.Parser.Parse:` 等の接頭辞をリーフに内包しており（約30箇所＋サブパーサ）、最深 Unwrap でも完全には消えない。`session new` に不正な `.puml` を渡した場合などに残る。パーサ全体の文言整理は本スコープ外の別タスク。

---

## 重大度 高

### H-1. Go の内部識別子がユーザー向けエラーに漏れている

エージェントが最も「未完成・cryptic」と感じた点。複数箇所でパッケージ名・関数名がそのまま露出する。

```console
$ csdfreplcmd statevar -json '[not json'
Error: csdf.SolveJSON: invalid JSON array: invalid character 'o' in literal null (expecting 'u')

$ csdfreplcmd statevar -json '{}'
Error: csdf.SolveJSON: invalid JSON array: top-level value must be an array

$ csdfreplcmd statevar -json '[null]'
Error: csdf.SolveJSON: null is not a supported JSON value

$ csdfreplcmd frobnicate
（usage を一通り出力した後）
tools.NewSubcommandFunc: tools.parseSubcommandOptions: no such subcommand
```

- **期待**: `csdf.SolveJSON:` `tools.NewSubcommandFunc:` のような実装内部の識別子は出さない。`[not json` のケースは Go 標準ライブラリ生の `invalid character 'o' in literal null (expecting 'u')` がそのまま出ており、JSON に不慣れな利用者には意味不明。
- **発生箇所**: `csdf/solver.go:62,69,73`、`tools/subcommand.go:85`
- **修正案**: ユーザー向け文言（例: `state variable values must be a JSON array, e.g. [["cola","water"]]`）に置き換え、内部識別子は `-debug` 時のみ。

### H-2. `session rm` が `-s` 無しで唯一のセッションを無確認削除する

```console
$ csdfreplcmd session rm
removed 2
```

- **期待**: `rm` は破壊的なので、`-s` も位置引数も無いときは usage かエラー、あるいは確認。README が一貫して `session rm -s "$SID"` と書いているので、引数なしで消えるのは想定外。
- **影響**: 今回は便利に働いたが、`read` のつもりで誤って `rm` と打つ等で唯一のセッションが消える footgun。「1セッションなら `-s` 省略可」という他コマンドの親切設計が、破壊的コマンドでは裏目に出ている。
- **修正案**: `session rm` は対象指定を必須にする（少なくとも複数セッション時のみ自動選択を許す）。

---

## 重大度 中

### M-1. 位置引数の後ろに置いたフラグが「2個目のインデックス」として誤解され、誤誘導するエラーになる

エージェントが自然な語順で打ったケース。

```console
$ csdfreplcmd select 0 -json
Error: select takes at most one index
```

- **期待**: `-json` はどの位置でも効く、または「フラグは index より前に置く」と分かるエラー。実際は Go の `flag` がインデックスで解析を打ち切り、`-json` を2個目の位置引数とみなしてこの文言になる。
- **影響**: 「インデックスは1個しか渡していないのに?」とミスリードされる。`csdfreplcmd select -json 0` で回避。`jump` 等インデックスを取る全コマンド共通の罠。
- **発生箇所**: `tools/csdfreplcmd/csdfreplcmdcmd/commands.go:123`

### M-2. `statevar` の個数不一致が「いくつ必要か」を示さない

状態変数 `availableProducts`（1個）に対し、商品2点を2要素と読み違えて打ったケース。

```console
$ csdfreplcmd statevar -json '["cola","water"]'
Error: State variable values length mismatch
```

- **期待**: `expected 1 value (availableProducts), got 2` のように期待個数と変数名を出す。正解は「1変数 = 1要素」で、各変数の値が配列なら入れ子 `[["cola","water"]]` が要る、という形が掴みづらい。
- **影響**: `read` をやり直して変数の個数を数え直させられた。
- **発生箇所**: `csdf/animation/proto/service.go:144`

### M-3. `select` / `jump` の範囲外エラーが有効範囲を示さない

```console
$ csdfreplcmd select 9
Error: Index out of range
$ csdfreplcmd jump 99
Error: Index out of range
```

- **期待**: `index out of range: valid 0–1`（遷移）/ `valid 0–6`（履歴）のように上限を提示。今は範囲を知るために `select`（一覧）や `history` に戻らないといけない。
- **発生箇所**: `csdf/animation/session.go:13`（`ErrIndexOutOfRange`）

### M-4. 負のインデックスがフラグとして解釈される

```console
$ csdfreplcmd jump -5
flag provided but not defined: -5
Usage of csdfreplcmd jump:
  ...
Error: flag provided but not defined: -5
```

- **期待**: `index must be >= 0`。`-5` が「未定義フラグ」になり、本来のエラー（不正なインデックス）から遠い。
- **発生箇所**: Go `flag` のデフォルト挙動（`commands.go` の各 parse で位置引数化される前に flag 解析される）

### M-5. グループの `-h` / `<group> help` が壊れている

リーフ（`read -h`, `select -h`, `session new -h`）は正常に help を出して exit 0。一方グループ階層が壊れている。

```console
$ csdfreplcmd session -h
Usage: csdfreplcmd session <command>
  ...
Error: session new requires exactly one file (.puml or .png)   ← new に落ちて exit 1

$ csdfreplcmd session help
  ...
tools.NewSubcommandFunc: tools.parseSubcommandOptions: no such subcommand   ← exit 1
```

- **期待**: `-h` / `help` は help を出して exit 0。`session -h` は usage を出した後に `new` 固有のエラーへ落ちて exit 1 になる。
- **発生箇所**: `tools/subcommand.go`（`help` を未知サブコマンド扱い、`-h` がフォールスルー）

---

## 重大度 低

### L-1. `help <subcommand>` がサブコマンド別ヘルプにならない

```console
$ csdfreplcmd help session     # 引数を無視してトップレベル help を再表示（exit 0）
```

- **期待**: `session` のヘルプ。サブコマンド別を見たいなら `session -h` を当てるしかなく、上記 M-5 で `session -h` も壊れているため逃げ場がない。

### L-2. ヘルプが3系統あり、書式・インデントがバラバラ

- `csdfreplcmd help`（自作 `helpCommand`）→ 整ったテーブル。
- 未知サブコマンド時や `session -h` の usage（`tools.NewSubcommandFunc` 自動生成）→ タブ混じりでインデントが崩れ、`session`/`read` で字下げが不揃い。
- リーフ `-h`（Go `flag` 既定）→ また別書式。

同じツールで3種類の見た目が混在し、どれが「正」か掴みにくい。**発生箇所**: `commands.go:36-46` vs `tools/subcommand.go`。

### L-3. `history` が分岐（jump）を区別せず、現在位置マーカーも無い

`jump 2` で `vmWaitingChoosing` に戻ってから歩くと、`history` に `[2]` と同一の trace+state が新エントリ `[5]` として複製され、「`[2]` から分岐」の注記が無い。現在のカーソル位置も表示されない。

- **期待**: jump で履歴を巻き戻す/切り詰める、または「branched from [2]」「← current」を表示。
- **影響**: 自分が履歴のどこにいるか、直前の `statevar` 出力からしか分からなかった。

### L-4. `session list` のテキスト出力がヘッダ無しの無名タブ区切り

```console
1	values	vmIdle	examples/valid/vending_machine.puml
```

- 4列が無ラベルのタブ区切り。2列目（`values` / `command`）は `-json` でしか名前が出ない `mode` フィールドで、列の意味を推測させられた。
- **期待**: ヘッダ行か、列の凡例。

### L-5. 状態表示が `name (id)` 固定で冗長

```
State: vmIdle (vmIdle)
```

name と id が同一のとき `vmIdle (vmIdle)` と二重表示。今回の仕様は全状態で name=id のため全行が冗長。name=id のときは片方に畳むと読みやすい。

### L-6. エラー文言の大文字小文字が不統一

`Index out of range`（先頭大文字, server 由来）/ `select takes at most one index`（小文字, client parse 由来）/ `State variable values length mismatch`（先頭大文字）が混在。Go 慣習（エラーは小文字始まり）に寄せると揃う。

---

## エージェント自身のつまずき（ツール起因でないが参考）

- **zsh の語分割の思い込み**: `for sub in ...; csdfreplcmd $sub` で `$sub="select -h"` を渡したが、zsh は未クォート変数を語分割しないため `"select -h"` が1引数として渡り、全件で「no such subcommand」usage が出た。エージェントは一瞬「`-h` は全サブコマンドで壊れている」と誤結論し、単体で打ち直して正常と判明。シェル側のミスだが、**未知サブコマンド時の usage dump がどのトークンを未知と判定したかを示さない**ため誤解を後押しした（L-2 と関連）。

---

## うまく動いていた点（壊さないように）

- **状態つきプロンプトが優秀**: 値を聞く前に `read` / `select` が遷移先の Post State Group・各変数（`availableProducts' as any`）・Guard・Post Condition を表示し、「何をどの順で入れるか」が明確。
- **操作後に結果状態＋遷移一覧を即再表示**するので、別途 `read` がほぼ不要。
- **`-json` 出力が単一行で綺麗にパースでき**（read/trace/history/session list/serverversion）、JSON `read` は `previous` 状態まで含む。
- **1セッション時は `-s` 省略可**（破壊的でない読み取り系では妥当）。
- **的確なエラーも多い**: `not awaiting values; select a transition first`、`not a natural number: "abc"`、`no such session: "999"`、`statevar accepts only one of -json or -json-file`、`statevar requires -json <json-text> or -json-file <file>`。
- **`statevar` は JSON 解析より先に mode を検査**するので、タイミングが悪いと JSON エラーでなく「先に遷移を選べ」が出る。
- **`session new` が素の id だけを出力**し、`SID=$(... session new ...)` でそのまま捕捉できる。
- **README の headless 節が正確**で、立ち上がりは速かった。

---

## 優先度の所感

エージェント運用での体感コストが高い順:

1. **H-1 内部識別子の漏れ** — 一番「壊れている/未完成」に見える。文言差し替えだけで印象が大きく改善。
2. **M-2 / M-3 エラーが期待値・有効範囲を出さない** — エージェントが `read` / `select` / `history` に往復させられる主因。エラー本文に必要情報を載せれば往復が消える。
3. **M-1 / M-4 フラグ・インデックス解析の罠** — 自然な打ち方が誤誘導エラーになる。
4. **M-5 / L-1 / L-2 ヘルプの不整合** — 自己探索の入口が安定しない。
5. **H-2 `session rm` の無確認削除** — 頻度は低いが事故ると痛い。
