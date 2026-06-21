# FDR の安定失敗詳細化アルゴリズム（調査結果）

本ドキュメントは、FDR（Failures-Divergences Refinement）における
**安定失敗詳細化 (stable failures refinement)** を判定するアルゴリズムを、
一次資料（出典 §8）に基づいて整理したもの。高速化のための状態圧縮は対象外とし、
**最も単純な版**を記述する。記憶どおり中核に**決定化（＝正規化, normalisation）**が入る。

設計上の方式選定（自然言語ガードの扱い）は `PLAN.md` を参照。本書はその土台となる
「制御レベルのアルゴリズム」に限定する。

---

## 1. 記法

- `Act`：可視イベントの集合。`τ`：内部（不可視）イベント。`Actτ = Act ∪ {τ}`。
- `s --a--> t`：1ステップ遷移（`a ∈ Actτ`）。
- `s =⇒ t`：**弱遷移**。τ を0回以上たどって `s` から `t` に到達（`τ*`）。
- `s =ρ=> t`：**弱トレース遷移**（`ρ ∈ Act*`）。`τ*` と可視イベントを交互にたどる（`τ* a₁ τ* a₂ … τ*`）。
- `enabled(s) = { a ∈ Actτ | ∃t. s --a--> t }`。
- `weaktraces(s) = { ρ ∈ Act* | ∃t. s =ρ=> t }`。

詳細化の対象（refined される側）を **Spec**、詳細化する側を **Impl** と呼ぶ。
FDR の向きは `Spec ⊑ Impl`（Spec が Impl によって詳細化される ＝ Impl のふるまいは
すべて Spec が許す）。

---

## 2. 安定失敗モデルの定義

- **安定 (stable)**：`stable(s) ⟺ τ ∉ enabled(s)`（内部遷移で動けない＝観測が落ち着いた）。
- **拒否集合 (refusals)**：安定な `s` について `refusals(s) = P(Act \ enabled(s))`。
  「`s` がどれも実行できないイベント集合」の全部分集合（下方閉）。
  状態集合 `U ⊆ S` については `refusals(U) = { X ⊆ Act | ∃s∈U. stable(s) ∧ X ∈ refusals(s) }`。
- **安定失敗 (stable failures)**：
  `failures(s) = { (ρ, X) ∈ Act* × P(Act) | ∃t. s =ρ=> t ∧ stable(t) ∧ X ∈ refusals(t) }`。
  「弱トレース `ρ` の後に安定状態へ到達し、そこで `X` を拒否できる」観測の集合。

### 詳細化関係
- **トレース詳細化**：`Spec ⊑tr Impl ⟺ weaktraces(Impl) ⊆ weaktraces(Spec)`。
- **安定失敗詳細化**：
  `Spec ⊑sfr Impl ⟺ failures(Impl) ⊆ failures(Spec) かつ weaktraces(Impl) ⊆ weaktraces(Spec)`。

> 注意：トレース包含の条項は**必須**。これを落とす実装が文献にあるが誤り（出典 §8 の antichain 論文の指摘）。

---

## 3. アルゴリズム全体像（3段）

`Spec ⊑sfr Impl` を**到達可能性問題**に帰着する。

1. **Spec を正規化（決定化）** → `norm(Spec)`（§4）。
2. **積 `norm(Spec) ⋈ Impl` を構築・探索**（§5）。
3. 各到達配置で**局所的に違反（witness）を判定**（§6）。

Spec のみ正規化し、Impl はそのまま積に入れる。探索ループ自体は単純な BFS（§7）。

---

## 4. 正規化（部分集合構成＝決定化）

`norm(L) = (S', ι', →')` を次で定義する。状態は**元の状態の集合** `U ⊆ S`：

- **状態集合**：`S' = P(S)`（べき集合）。
- **初期状態**：`ι' = { s ∈ S | ι =⇒ s }`（Spec 初期状態の **τ閉包**）。
- **遷移**：可視イベント `a ∈ Act` のみ。
  `U --a-->' V ⟺ V = { t ∈ S | ∃s ∈ U. s =a=> t }`（弱トレース遷移で集める）。

性質：`norm(L)` は **決定的・τなし (concrete)・全域的 (universal)**。
ある `a` で行き先が空になるとき、状態は `∅` になる（`∅` も状態の一つ）。

- `∅` の意味：`ρ` が `weaktraces(L)` に**無い** ⟺ `ι'` から `ρ` で `∅` に到達。

> **決定化との違い**：正規化は元 LTS に**無かった弱トレースを増やしうる**
> （拒否情報のために状態集合を保持するため）。
> このため各正規形状態 `U` は、対応する**元の状態集合**（本書では `[[U]]` と書く）を
> 保持しておく必要がある。拒否集合 `refusals([[U]])` の計算に使う。

---

## 5. 積（product）と探索

積 `norm(Spec) ⋈ Impl` の遷移規則：

- Impl の `τ`：`(U, i) --τ--> (U, i')`（Spec 側 `U` は不動。正規形に τ は無いため）。
- 可視 `a`：`(U, i) --a--> (U', i')`（両者同時。`U --a-->' U'` かつ `i --a--> i'`）。

初期配置 `(ι', root(Impl))` から到達可能な配置を BFS で網羅する。
BFS により、違反時に**最小の反例トレース**が得られる。

---

## 6. 違反（SF-witness）の局所判定

到達配置 `(U, i)` が **SF-witness（＝詳細化違反）** であるとは、次のいずれか：

1. `U = ∅`（Impl のトレースが Spec に無い → **トレース違反**）、**または**
2. `stable(i)` かつ `refusals(i) ⊄ refusals([[U]])`（**拒否違反**）。

**定理**：`Spec ⊑sfr Impl ⟺ 積 norm(Spec) ⋈ Impl に到達可能な SF-witness が存在しない`。
（トレース詳細化は「TR-witness ＝ `U=∅` が到達不能」と同値。）

### 実装しやすい形（受理集合＝メニュー）
拒否集合は下方閉なので、条件 2 は **`i` の最大拒否 `Act \ enabled(i)` が `refusals([[U]])` に
入るか**だけ見れば十分。式変形すると：

```
refusals(i) ⊆ refusals([[U]])
  ⟺ (Act \ enabled(i)) ∈ refusals([[U]])
  ⟺ ∃ 安定な s ∈ [[U]]. enabled(s) ⊆ enabled(i)
```

よって：

> **`(U, i)` で違反なし ⟺ `i` が不安定、または `[[U]]` の中に
> `enabled(s) ⊆ enabled(i)` を満たす安定状態 `s` が少なくとも1つ存在。**
>
> **`(U, i)` が SF-witness ⟺ `U = ∅`、または
> （`i` が安定 ∧ `[[U]]` のどの安定状態 `s` も `enabled(s) ⊄ enabled(i)`）。**

直感：安定な Impl 状態 `i` がメニュー `A_i = enabled(i)` を出すなら、Spec 集合 `[[U]]` の中に
「メニューが `A_i` の部分集合になる安定状態」が必要（Spec も同じだけ拒否できる必要がある）。
ここで `enabled(·)` は可視イベントのみを指す（安定状態には τ が無い）。

> FDR は実装上、各正規形状態に**極小受理集合 (minimal acceptances)** を持たせ、
> その一つが `enabled(i)` に含まれるかで判定する。これは上記と等価。

---

## 7. 単一スレッド擬似コード（FDR3, Figure 1 相当）

```
function Refines(S, I, M):          # S = 正規化済み Spec, I = Impl, M = 意味モデル
  done    = {}
  current = { (root(S), root(I)) }
  next    = {}
  while current ≠ {}:
    for (s, i) in current \ done:
      「i が s を M の意味で詳細化しているか」を局所判定   # ← モデル依存はここだけ（§6）
      done += (s, i)
      for (e, i') in transitions(I, i):
        if e == τ:
          next += (s, i')                 # Spec は動かない
        else:
          t = transitions(S, s, e)
          if t == {}:
            Report trace error            # Spec がその e を持てない（U=∅ 相当）
          else:
            {s'} = t                      # 正規形は決定的なので後継は一意
            next += (s', i')
    current = next
    next = {}
```

- `M = traces`：局所判定は不要（`trace error` がトレース違反を担う）。
- `M = stable failures`：局所判定 ＝ §6 の拒否チェック（安定 `i` で `enabled(s') ⊆ enabled(i)` を
  満たす極小受理集合が `s'` に在るか）。

---

## 8. 退化ケース：τ が無い（hiding が無い）とき

合成後 LTS に τ（内部遷移）が無い場合、上記は大きく単純化する。本リポジトリのモデルでは
`event` が厳密に `tau` のときのみ τ が生じる（`docs/SYNTAX.md`）。τ を含まないサブセットでは：

- `=⇒` は恒等、`=a=>` は1ステップ遷移、**全状態が安定**。
- 正規化 ＝ 素朴な部分集合決定化（τ閉包不要）。
- 違反判定 `(U, i)`：`U = ∅`、または `∀ s ∈ U. enabled(s) ⊄ enabled(i)`。
- 非決定性は **Spec 図に同一イベントの多重辺がある**場合に生じる → だから決定化が要る。

これが「単純さ最優先」の最初の到達点であり、τ・発散 (divergence) は段階的に積み増せる。

---

## 9. 本リポジトリのデータ構造との対応

- `csdf.Diagram`（`States`, `Edges{Src,Dst,Event}`, `StartEdge`）が LTS に相当。
- `ComposeParallel` が Impl 側の合成 LTS を作る既存経路。
- 追加で必要になる概念：①正規形状態（`Set[StateID]` ＋ 極小受理集合）、
  ②`normalize(Diagram) → NormalForm`、③`refinesSF(spec, impl) → (ok, 反例トレース)` の BFS。
- 既存の `StatePair` / `Trans`（`composition.go`）は積探索にそのまま流用できる。

> ガード `Guard` と事後条件 `Post` が自然言語である点の扱いは、本書のスコープ外（`PLAN.md` 参照）。

---

## 10. 出典

- Gibson-Robinson, Armstrong, Boulgakov, Roscoe,
  *FDR3 — A Modern Refinement Checker for CSP* — 正規化の位置づけと
  単一スレッド詳細化アルゴリズム（Figure 1）。
- Laveaux, Groote, Willemse,
  *Correct and Efficient Antichain Algorithms for Refinement Checking*（arXiv:1902.09880）—
  拒否・安定失敗・正規形・積・SF-witness の厳密定義と正しさの証明。
- *FDR2 User Manual: CSP Refinement* — 正規化と詳細化の概説。
