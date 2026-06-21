結論として、有限 LTS に対する FDR アルゴリズムは妥当です。問題は自然言語・状態変数を含む記号モデルへの拡張部分です。現状の `PLAN.md` のままでは、案4を含めて健全な判定器にはなりません。

| 方法 | 健全性 | 実現可能性 | 評価 |
|---|---:|---:|---|
| 案1 記号的VC | 現状 ×／修正後 ○ | △ | 方針は良いが、VC定式化に欠陥 |
| 案2 オラクル | 一般には × | 限定用途 ○ | 有限モデルの探索器・反例候補生成向け |
| 案3 事前形式化 | 翻訳モデルに対して ○ | △ | 制限された形式言語なら有望 |
| 案4 ハイブリッド | 現状 ×／再設計後 ○ | ○ | 最有力だが層の責務を修正すべき |

## 案1：記号的VC

健全性は現状の記述では成立しません。

最大の問題は O-refusal です。[PLAN.md:85](./PLAN.md#L85) は enabledness を Guard だけで判定していますが、資料自身の遷移定義では、辺が有効なのは次です。

```text
Enabled_e(v)
  = OR(edge.event = e)
      Guard_edge(v) ∧ ∃v'. Post_edge(v, v')
```

したがって、拒否 VC も `GuardS ⇒ GuardI` ではなく、イベント単位の `EnabledS ⇒ EnabledI` でなければなりません。

例えば Spec の `a` 辺が実行可能、Impl の `a` 辺が `Post=false` なら、Impl は `a` を拒否できます。しかし現在の式は両方の Guard が `true` なら通過してしまいます。同様に安定性も、τ の Guard ではなく「実行可能な τ 後続が存在しない」で判定する必要があります。

また、[PLAN.md:68](./PLAN.md#L68) の「制御レベルで回して有限個の VC を生成」も一般には成立しません。ループがあれば経路は無限です。有限化できるのは、積の各制御状態に対する帰納的不変条件を別途与える場合です。

評価：

- 十分条件を証明する仕組みとしては実現可能。
- 完全な判定器としての記号的正規化は、無限状態・不動点・量化消去があるため実現性が低い。
- `Inv` が単一 Spec valuation を表すなら、非決定的 Spec に対する完全性も失われます。正規形状態は「Spec configuration の集合」として表す必要があります。

## 案2：オンデマンド・オラクル

一般的な検証器としては健全ではありません。

有限領域であっても、健全になる条件は単に「列挙可能」ではなく、オラクルが次を完全かつ正確に返すことです。

- 全初期 valuation
- 全後続 valuation
- Guard/Post の正確な真偽
- τ 後続を含む完全な閉包

LLMや人間への点ごとの問い合わせでは、後続の取りこぼしを検出できません。キャッシュは再現性を改善しますが、回答の正しさは改善しません。

実装面では限定用途なら容易です。ただし既存の `PostSolver` は Guard/Post を受け取るだけで、実際には評価していません。[solver.go:56](./csdf/solver.go#L56) と README にもその制約があります。

適した位置づけは次です。

- 有界探索
- 具体例のシミュレーション
- 反例候補生成
- 結果を `PASS` ではなく `UNKNOWN` または `CANDIDATE_FAILURE` として返す補助機能

## 案3：LLMによる事前形式化

形式化後のモデルに対しては健全にできます。しかし「自然言語の意図に対する健全性」とは別です。

もう一つ重要なのは、[PLAN.md:145](./PLAN.md#L145) の「決定可能理論に収まればアルゴリズムも完全」という記述です。これは成立しません。

個々の SMT 式が決定可能でも、無限状態遷移系の到達可能性や不動点計算が決定可能とは限りません。追加で以下のいずれかが必要です。

- 有限 valuation
- 有限抽象化・有限商
- 到達可能性が決定可能な限定モデル
- 有界検査
- ユーザー提供の帰納的不変条件

実現可能性は対象を限定すれば高まります。「任意の自然言語」ではなく、整数・列挙型・論理演算などの明示的な DSL を決め、LLMにはその DSL の候補を生成させ、人間が承認する方式が現実的です。

## 案4：ハイブリッド

アーキテクチャとしては最も良い方向ですが、現在の Layer A は成立しません。

[PLAN.md:157](./PLAN.md#L157) は正規化と τ 閉包を「NLなし」で計算するとしています。しかし次は Guard/Post の意味を知らなければ計算できません。

- 辺が実行可能か
- τ 辺が実行可能か
- τ 閉包
- stable か
- 弱後像
- 受理集合

構文上存在する全辺を採用すると、Spec の偽の遷移が違反を隠す可能性があり、健全な `PASS` を返せません。

修正版は次の構成が妥当です。

1. FDR コア  
   有限の明示的 LTS に対する正規化・積・SF-witness探索。ここは完全にテスト可能。

2. 意味論バックエンド  
   `Enabled`、`Successors`、`TauClosure`、`Valid` などを提供。答えられない場合は `Unknown` を返す。

3. 形式化層  
   DSL、SMT、有限列挙など。LLMは形式化案を生成するが、証明器としては扱わない。

4. 判定結果  
   `PASS / FAIL / UNKNOWN` の三値。  
   `PASS` はすべて証明済み、`FAIL` は実行可能性を確認した具体的 witness がある場合だけ返す。

## 先に確定すべき横断的事項

方式選定前に以下を仕様化すべきです。

- `StartEdge.Post` が作る複数の初期 valuation の扱い
- `EndEdge` を CSP の成功終了イベント `✓` とするのか、単なる停止とするのか
- Post が充足不能な辺は disabled とすること
- 同一イベントの複数辺に対する enabledness の論理和
- 状態ごとに異なる変数のスコープと合成時の名前衝突
- 発散を許した stable failures と、divergence-free 前提との区別

特に `EndEdge` は AST にありますが、現在の並列合成では明示的に未対応です。[composition.go:55](./csdf/composition.go#L55)

## 推奨

案4を採用する方向には賛成ですが、実装順序は次が安全です。

1. Guard/Post がすべて `true` の有限 LTS で FDR コアをTDD実装
2. 有限列挙 valuation の厳密バックエンド
3. 三値判定と証明課題 IR
4. 制限された形式 DSL＋SMT
5. 最後に LLM 形式化支援

基礎となる有限 LTS の定理自体は、正規化 Spec と Impl の積に SF-witness が存在しないことと stable failures refinement が同値であると証明されています。FDR3 も、Spec を τ のない決定的 GLTS に正規化してから refinement を検査する構造です。

### Sources

- [Correct and Efficient Antichain Algorithms for Refinement Checking](https://lmcs.episciences.org/7143/pdf)
- [FDR3 — A Modern Refinement Checker for CSP](https://www.cs.ox.ac.uk/files/6001/Document.pdf)
- [FDR2 Technical Details](https://www.cs.ox.ac.uk/projects/concurrency-tools/fdr-2.94-html-manual/fdr2manual_28.html)
