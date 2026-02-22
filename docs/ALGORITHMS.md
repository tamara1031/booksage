# BookSage 推論アルゴリズム視覚化ドキュメント

本ドキュメントでは、Agentic RAGシステム「BookSage」における推論アルゴリズムのフローを解説します。
システムは **Go API Orchestrator** を中心とした設計となっており、全てのLLM推論（生成、Embedding）はGoサービスが管理します。Python Workerはドキュメントの構造解析のみを担当し、推論処理は行いません。

## 1. Ingest（取り込み）時の推論アルゴリズム

ドキュメントのアップロードから、ベクトルストア（Qdrant）およびグラフデータベース（Neo4j）への保存までのフローです。
階層的な要約生成（RAPTOR）とエンティティ抽出（GraphRAG）を組み合わせ、多角的なインデックスを構築します。

```mermaid
sequenceDiagram
    participant Client
    participant Go as Go Orchestrator
    participant Py as Python Worker
    participant LLM as Ollama (Local)
    participant Vec as Qdrant (Vector)
    participant Graph as Neo4j (Graph)

    Client->>Go: ドキュメントアップロード (PDF/Markdown等)
    activate Go

    %% 解析フェーズ
    Note right of Go: [解析] 構造化データ抽出
    Go->>Py: Parse Request (Docling)
    activate Py
    Py-->>Go: 階層ツリー構造 & チャンク返却
    deactivate Py

    %% 要約推論フェーズ (RAPTOR)
    rect rgb(240, 248, 255)
        Note right of Go: [要約推論 - RAPTOR]<br/>再帰的要約生成
        loop 下位チャンクから上位セクションへ
            Go->>LLM: Summarize Request
            activate LLM
            LLM-->>Go: Summary Text
            deactivate LLM
        end
    end

    %% 抽出推論フェーズ (GraphRAG)
    rect rgb(255, 240, 245)
        Note right of Go: [抽出推論 - GraphRAG]<br/>エンティティ・関係性抽出
        Go->>LLM: Extract Entities & Relations
        activate LLM
        LLM-->>Go: Entities JSON
        deactivate LLM
    end

    %% ベクトル化フェーズ
    rect rgb(240, 255, 240)
        Note right of Go: [ベクトル推論]<br/>Embedding生成
        Go->>LLM: Generate Embeddings (Chunks & Entities)
        activate LLM
        LLM-->>Go: Vectors
        deactivate LLM
    end

    %% 保存フェーズ
    Note right of Go: [名寄せと保存]
    Go->>Vec: コサイン類似度による名寄せ判定
    Go->>Vec: ベクトル保存 (Upsert)
    Go->>Graph: インクリメンタル保存 (GT-Link: TreeノードとEntityの結合)

    Go-->>Client: Ingest完了通知
    deactivate Go
```

### アルゴリズム詳細

1.  **[解析] Python WorkerによるDoclingパース**
    -   Go OrchestratorからPython Workerへドキュメントデータを送信します。
    -   Python Workerは `Docling` 等のライブラリを用いてドキュメントを解析し、章・節・項の階層構造（Tree）と、最小単位のテキストチャンクを抽出してGoへ返却します。
    -   *制約事項*: Python WorkerはLLMを一切呼び出しません。

2.  **[要約推論 - RAPTOR] 再帰的要約**
    -   Go Orchestratorは抽出されたチャンクを受け取り、Ollamaを呼び出します。
    -   **RAPTOR (Recursive Abstractive Processing for Tree-Organized Retrieval)** アルゴリズムに基づき、下位のチャンク群を要約して上位のノード（セクション要約）を生成します。これをルートノードまで再帰的に繰り返すことで、ドキュメントの全体像を捉えたコンテキストを生成します。

3.  **[抽出推論 - GraphRAG] エンティティと関係性の抽出**
    -   Go Orchestratorは各チャンクに対してOllamaを呼び出し、テキスト内に含まれる重要な「エンティティ（人物、場所、概念など）」と、それらの間の「関係性」を抽出します。
    -   これにより、キーワード検索だけでは捉えきれない意味的なつながりをグラフデータとして構築します。

4.  **[ベクトル推論] Embedding生成**
    -   抽出されたテキストチャンク、生成された要約、および抽出されたエンティティに対して、Go OrchestratorがOllama（Embeddingモデル）を呼び出し、ベクトル表現を生成します。

5.  **[名寄せと保存] Qdrant & Neo4jへの保存**
    -   **Qdrant**: 生成されたベクトルを用い、既存データとのコサイン類似度を計算して名寄せ（重複排除）を行った上で保存します。
    -   **Neo4j**: ドキュメントの階層構造（Treeノード）と抽出されたエンティティを保存します。この際、**GT-Link (Graph-Tree Link)** アルゴリズムにより、構造ツリーと意味グラフを相互に結合し、横断的な検索を可能にします。

---

## 2. Inference（検索・生成）時の推論アルゴリズム

ユーザーからのクエリを受け取り、最適な回答を生成するまでのフローです。
適応的なルーティング、多角的な検索、および自己評価ループを備えています。

```mermaid
sequenceDiagram
    participant Client
    participant Go as Go Orchestrator
    participant LLM as Ollama (Local)
    participant Vec as Qdrant (Vector)
    participant Graph as Neo4j (Graph)

    Client->>Go: ユーザー検索クエリ
    activate Go

    %% ルーティング
    rect rgb(255, 250, 205)
        Note right of Go: [ルーティング推論]<br/>Adaptive Routing
        Go->>LLM: クエリ複雑度判定
        activate LLM
        LLM-->>Go: Simple / Complex / Agentic
        deactivate LLM
    end

    %% キーワード抽出
    rect rgb(230, 230, 250)
        Note right of Go: [キーワード抽出推論 - LightRAG]<br/>Dual-level Retrieval
        Go->>LLM: Extract Low/High-level Keys
        activate LLM
        LLM-->>Go: Specific Entities & Broad Themes
        deactivate LLM
    end

    %% 並行探索
    par Parallel Search
        Go->>Vec: ベクトル検索 (Dense Retrieval)
        and
        Go->>Graph: グラフ探索 (Cypher Query)
    end
    Vec-->>Go: 類似チャンク
    Graph-->>Go: 関連エンティティ・関係性

    %% ランク付け
    Note right of Go: [アルゴリズム評価 - Skyline Ranker]<br/>パレート最適 (Vectorスコア vs Graphスコア)
    Go->>Go: コンテキストの厳選・フィルタリング

    %% 生成と自己評価
    rect rgb(255, 228, 225)
        loop Self-RAG Loop
            Note right of Go: [生成と自己評価推論 - Self-RAG]
            Go->>LLM: 回答生成 (with Context)
            activate LLM
            LLM-->>Go: 回答ドラフト
            deactivate LLM

            Go->>LLM: 自己評価 (Critique / Fact Check)
            activate LLM
            LLM-->>Go: Score / Pass / Fail
            deactivate LLM

            alt 基準未達 (Fail)
                Go->>Go: クエリ/コンテキスト修正してリトライ
            else 基準達成 (Pass)
                Go-->>Client: 最終回答
            end
        end
    end
    deactivate Go
```

### アルゴリズム詳細

1.  **[ルーティング推論] Adaptive Routing**
    -   Go Orchestratorはユーザーのクエリを受け取ると、まずOllamaを呼び出してクエリの性質を分析します。
    -   クエリが単純な事実確認か、複雑な推論を要するか、あるいはマルチステップの処理が必要か（Agentic）を判定し、後続の処理パイプラインを動的に切り替えます。

2.  **[キーワード抽出推論 - LightRAG] Dual-level Retrieval**
    -   Go OrchestratorはOllamaを呼び出し、クエリから検索用のキーワードを抽出します。
    -   **LightRAG** のアプローチを採用し、「Low-level keys（具体的なエンティティ名など）」と「High-level keys（抽象的なテーマや概念）」を同時に抽出することで、局所的な情報と大局的な文脈の両方を検索対象とします。

3.  **[並行探索] Vector & Graph Parallel Search**
    -   抽出されたキーワードを用い、Goの `Goroutines` を使用してQdrant（ベクトル検索）とNeo4j（グラフ探索）を並行して検索します。
    -   これにより、意味的な類似性と構造的な関連性の両面から候補となるコンテキストを収集します。

4.  **[アルゴリズム評価] Skyline Ranker**
    -   収集された膨大なコンテキスト候補に対し、Go内部のロジックでフィルタリングを行います。
    -   **Skyline Ranker** アルゴリズムを適用し、「ベクトル類似度スコア」と「グラフ関連度スコア」の2軸で評価します。パレート最適（どちらのスコアも他の候補より劣っていない解）なコンテキストのみを厳選し、LLMのコンテキストウィンドウを効率的に利用します。

5.  **[生成と自己評価推論 - Self-RAG] Generation & Critique**
    -   厳選されたコンテキストと共にOllamaへプロンプトを送信し、回答を生成します。
    -   生成された回答に対し、即座にOllamaを用いて「自己評価（Critique）」を行います。回答が事実に基づいているか（Faithfulness）、クエリに答えているか（Relevance）を検証します。
    -   基準を満たさない場合、Go Orchestratorは検索クエリを修正したりコンテキストを再選定したりして、回答生成をリトライします（Self-Correction Loop）。
